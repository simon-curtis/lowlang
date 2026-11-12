package vkgen

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"lowlang/internal/compiler"
	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/shaders"
	"lowlang/internal/target"
)

var root = filepath.Join("..", "..", "ui")

var registry = sync.OnceValues(func() (*Registry, error) {
	data, err := os.ReadFile(filepath.Join(root, "registry", "vk.xml.gz"))
	if err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	xml, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return Parse(xml)
})

func generate(t *testing.T) string {
	t.Helper()
	reg, err := registry()
	if err != nil {
		t.Fatal(err)
	}
	list, err := os.ReadFile(filepath.Join(root, "registry", "vulkan.list"))
	if err != nil {
		t.Fatal(err)
	}
	sel, err := ParseSelection(string(list))
	if err != nil {
		t.Fatal(err)
	}
	source, err := Generate(reg, sel)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestParseDecl(t *testing.T) {
	cases := []struct {
		inner string
		want  Decl
	}{
		{`const <type>VkApplicationInfo</type>* <name>pApplicationInfo</name>`, Decl{Const: true, Type: "VkApplicationInfo", Stars: 1, Name: "pApplicationInfo"}},
		{`<type>char</type> <name>deviceName</name>[<enum>VK_MAX_PHYSICAL_DEVICE_NAME_SIZE</enum>]<comment>x</comment>`, Decl{Type: "char", Name: "deviceName", Dims: []string{"VK_MAX_PHYSICAL_DEVICE_NAME_SIZE"}}},
		{`<type>float</type> <name>matrix</name>[3][4]`, Decl{Type: "float", Name: "matrix", Dims: []string{"3", "4"}}},
		{`<type>VkBool32</type> <name alias="X::y">robustImageAccess</name>`, Decl{Type: "VkBool32", Name: "robustImageAccess"}},
		{`<type>void</type>** <name>ppData</name>`, Decl{Type: "void", Stars: 2, Name: "ppData"}},
	}
	for _, c := range cases {
		got, err := ParseDecl(c.inner)
		if err != nil {
			t.Fatal(err)
		}
		if got.Const != c.want.Const || got.Type != c.want.Type || got.Stars != c.want.Stars || got.Name != c.want.Name || strings.Join(got.Dims, ",") != strings.Join(c.want.Dims, ",") {
			t.Errorf("ParseDecl(%q) = %+v, want %+v", c.inner, got, c.want)
		}
	}
}

func TestEnumValues(t *testing.T) {
	cases := []struct {
		enum Enum
		ext  int
		want int64
	}{
		{Enum{Offset: "4", ExtNumber: "315"}, 0, 1000314004},
		{Enum{Offset: "0"}, 2, 1000001000},
		{Enum{Offset: "1", Dir: "-"}, 1, -1000000001},
		{Enum{BitPos: "10"}, 0, 1024},
		{Enum{Value: "0x0000000FULL"}, 0, 15},
		{Enum{Value: "(~0U)"}, 0, -1},
	}
	for _, c := range cases {
		got, ok, err := enumValue(c.enum, c.ext)
		if err != nil || !ok || got != c.want {
			t.Errorf("enumValue(%+v) = %d %v %v, want %d", c.enum, got, ok, err, c.want)
		}
	}
	if literal(-1, "u32") != "4294967295" {
		t.Error("u32 literal is not unsigned")
	}
}

func TestSelection(t *testing.T) {
	sel, err := ParseSelection("# bindings\nextension VK_KHR_surface\n\ncommand vkCreateInstance\ntype VkExtent2D\n")
	if err != nil || len(sel.Extensions) != 1 || len(sel.Commands) != 1 || len(sel.Types) != 1 {
		t.Fatalf("ParseSelection = %+v, %v", sel, err)
	}
	if _, err := ParseSelection("function vkCreateInstance"); err == nil {
		t.Fatal("expected an error for an unknown entry")
	}
}

func TestUnknownCommand(t *testing.T) {
	reg, err := registry()
	if err != nil {
		t.Fatal(err)
	}
	_, err = Generate(reg, Selection{Commands: []string{"vkDoesNotExist"}})
	if err == nil || !strings.Contains(err.Error(), "unknown command vkDoesNotExist") {
		t.Fatalf("err = %v", err)
	}
}

func TestBindingsShape(t *testing.T) {
	source := generate(t)
	for _, want := range []string{
		"pub fn vkCreateInstance(mut pCreateInfo: VkInstanceCreateInfo, pAllocator: ptr[VkAllocationCallbacks], mut pInstance: VkInstance) -> i32",
		"pub fn vkMapMemory(device: VkDevice, memory: VkDeviceMemory, offset: i64, size: i64, flags: u32, mut ppData: ptr) -> i32",
		"    pub sType: i32 = 1000314004",
		"pub extern struct VkBuffer {\n    pub handle: i64 = 0\n}",
		"pub extern struct VkInstance {\n    pub handle: ptr = null\n}",
		"pub fn VkPhysicalDeviceProperties_deviceName(s: VkPhysicalDeviceProperties) -> str {",
		"pub fn VkPhysicalDeviceMemoryProperties_memoryTypes(s: VkPhysicalDeviceMemoryProperties) -> []VkMemoryType {",
		"pub fn VK_QUEUE_FAMILY_IGNORED() -> u32 {\n    4294967295\n}",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("generated bindings lack %q", want)
		}
	}
}

func TestOffscreenRender(t *testing.T) {
	fixturetest.RequireLibrary(t, "vulkan")
	dir := t.TempDir()
	program, err := os.ReadFile(filepath.Join("testdata", "offscreen.low"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"vulkan.low": []byte(generate(t)), "main.low": program}
	shaderDir := filepath.Join(root, "shaders")
	stages, err := shaders.Stages(shaderDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range stages {
		name := shaders.Binary(stage)
		data, err := os.ReadFile(filepath.Join(shaderDir, name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = data
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	unit, err := compiler.Analyse(dir, target.Linux)
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "offscreen")
	ctx := context.Background()
	if _, err := compiler.Build(ctx, unit, compiler.Release, binary); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(ctx, binary).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if strings.HasPrefix(string(out), "no vulkan 1.3 device") {
		t.Skip("no Vulkan 1.3 device")
	}
	want := "0 0 0 255\n255 0 0 255\n0 0 255 255\n0 0 0 255\n"
	if string(out) != want {
		t.Fatalf("pixels:\n%s\nwant:\n%s", out, want)
	}
}
