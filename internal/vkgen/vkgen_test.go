package vkgen

import (
	"bytes"
	"context"
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

var (
	bindingsDir = filepath.Join("..", "compiler", "std", "ui", "gpu")
	shaderDir   = filepath.Join(bindingsDir, "shaders")
)

const regenerate = "go run ./cmd/vkgen -registry internal/vkgen/registry/vk.xml.gz -select internal/vkgen/registry/vulkan.list -o internal/compiler/std/ui/gpu"

var registry = sync.OnceValues(func() (*Registry, error) {
	return ReadRegistry(filepath.Join("registry", "vk.xml.gz"))
})

func generate(t *testing.T) []File {
	t.Helper()
	reg, err := registry()
	if err != nil {
		t.Fatal(err)
	}
	list, err := os.ReadFile(filepath.Join("registry", "vulkan.list"))
	if err != nil {
		t.Fatal(err)
	}
	sel, err := ParseSelection(string(list))
	if err != nil {
		t.Fatal(err)
	}
	files, err := Generate(reg, sel)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestCheckedInBindings(t *testing.T) {
	for _, f := range generate(t) {
		got, err := os.ReadFile(filepath.Join(bindingsDir, f.Name))
		if err != nil {
			t.Fatalf("%v; run %s", err, regenerate)
		}
		if string(got) != f.Source {
			t.Errorf("%s differs from the generator output; run %s", f.Name, regenerate)
		}
	}
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
	sel, err := ParseSelection("# bindings\nlibrary linux vulkan\nlibrary windows vulkan-1\nextension VK_KHR_surface\n\ncommand vkCreateInstance\ncommand vkCreateWaylandSurfaceKHR linux\ntype VkExtent2D\n")
	if err != nil || len(sel.Extensions) != 1 || len(sel.Commands) != 2 || len(sel.Types) != 1 {
		t.Fatalf("ParseSelection = %+v, %v", sel, err)
	}
	if sel.Commands[0] != (SelectedCommand{Name: "vkCreateInstance"}) || sel.Commands[1] != (SelectedCommand{Name: "vkCreateWaylandSurfaceKHR", System: "linux"}) {
		t.Errorf("commands = %+v", sel.Commands)
	}
	if sel.Libraries["linux"] != "vulkan" || sel.Libraries["windows"] != "vulkan-1" {
		t.Errorf("libraries = %v", sel.Libraries)
	}
}

func TestSelectionErrors(t *testing.T) {
	cases := []struct{ text, want string }{
		{"function vkCreateInstance", "line 1: expected"},
		{"command", "line 1: expected"},
		{"command vkCreateInstance linux extra", "line 1: expected"},
		{"library linux", "line 1: expected"},
		{"library linux vulkan extra", "line 1: expected"},
		{"command vkCreateInstance plan9", `line 1: unknown system "plan9"`},
		{"\nlibrary plan9 vulkan", `line 2: unknown system "plan9"`},
		{"library linux vulkan\nlibrary linux vulkan-2", "line 2: library for linux is already named"},
	}
	for _, c := range cases {
		_, err := ParseSelection(c.text)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("ParseSelection(%q) error = %v, want %q", c.text, err, c.want)
		}
	}
}

func TestMissingLibrary(t *testing.T) {
	reg, err := registry()
	if err != nil {
		t.Fatal(err)
	}
	sel := Selection{Commands: []SelectedCommand{{Name: "vkCreateInstance"}}, Libraries: map[string]string{"linux": "vulkan"}}
	_, err = Generate(reg, sel)
	if err == nil || !strings.Contains(err.Error(), `no "library windows NAME" line`) {
		t.Fatalf("err = %v", err)
	}
}

func TestUnknownCommand(t *testing.T) {
	reg, err := registry()
	if err != nil {
		t.Fatal(err)
	}
	sel := Selection{Commands: []SelectedCommand{{Name: "vkDoesNotExist"}}, Libraries: map[string]string{"linux": "vulkan", "windows": "vulkan-1"}}
	_, err = Generate(reg, sel)
	if err == nil || !strings.Contains(err.Error(), "unknown command vkDoesNotExist") {
		t.Fatalf("err = %v", err)
	}
}

func TestTargetFiles(t *testing.T) {
	files := generate(t)
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	if want := "vulkan.low vulkan_linux.low vulkan_windows.low"; strings.Join(names, " ") != want {
		t.Fatalf("files = %v, want %s", names, want)
	}
	shared, linux, windows := files[0].Source, files[1].Source, files[2].Source
	if strings.Contains(shared, "extern \"") {
		t.Error("shared file holds an extern block")
	}
	for _, want := range []string{`extern "vulkan" {`, "fn vkCreateInstance(", "fn vkCreateWaylandSurfaceKHR(", "fn vkGetPhysicalDeviceWaylandPresentationSupportKHR("} {
		if !strings.Contains(linux, want) {
			t.Errorf("linux file lacks %q", want)
		}
	}
	for _, want := range []string{`extern "vulkan-1" {`, "fn vkCreateInstance(", "fn vkCreateWin32SurfaceKHR(", "fn vkGetPhysicalDeviceWin32PresentationSupportKHR("} {
		if !strings.Contains(windows, want) {
			t.Errorf("windows file lacks %q", want)
		}
	}
	if strings.Contains(linux, "Win32") {
		t.Error("linux file holds Win32 commands")
	}
	if strings.Contains(windows, "Wayland") {
		t.Error("windows file holds Wayland commands")
	}
	for _, want := range []string{"extern struct VkWaylandSurfaceCreateInfoKHR {", "extern struct VkWin32SurfaceCreateInfoKHR {"} {
		if !strings.Contains(shared, want) {
			t.Errorf("shared file lacks %q", want)
		}
	}
}

func TestBindingsShape(t *testing.T) {
	files := generate(t)
	var all strings.Builder
	for _, f := range files {
		all.WriteString(f.Source)
	}
	text := all.String()
	if strings.Contains(text, "pub ") {
		t.Error("generated bindings declare pub items")
	}
	for _, want := range []string{
		"fn vkCreateInstance(mut pCreateInfo: VkInstanceCreateInfo, pAllocator: ptr[VkAllocationCallbacks], mut pInstance: VkInstance) -> i32",
		"fn vkMapMemory(device: VkDevice, memory: VkDeviceMemory, offset: i64, size: i64, flags: u32, mut ppData: ptr) -> i32",
		"    sType: i32 = 1000314004",
		"extern struct VkBuffer {\n    handle: i64 = 0\n}",
		"extern struct VkInstance {\n    handle: ptr = null\n}",
		"fn VkPhysicalDeviceProperties_deviceName(s: VkPhysicalDeviceProperties) -> str {",
		"fn VkPhysicalDeviceMemoryProperties_memoryTypes(s: VkPhysicalDeviceMemoryProperties) -> []VkMemoryType {",
		"fn VK_QUEUE_FAMILY_IGNORED() -> u32 {\n    4294967295\n}",
		"    hwnd: ptr = null",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated bindings lack %q", want)
		}
	}
}

func offscreenDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	program, err := os.ReadFile(filepath.Join("testdata", "offscreen.low"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"main.low": program}
	for _, f := range generate(t) {
		files[f.Name] = []byte(f.Source)
	}
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
	return dir
}

func TestOffscreenRender(t *testing.T) {
	fixturetest.RequireLibrary(t, "vulkan")
	dir := offscreenDir(t)
	unit, err := compiler.Analyse(dir, compiler.Release(target.Linux))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "offscreen")
	ctx := context.Background()
	if _, err := compiler.Build(ctx, unit, binary); err != nil {
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

func TestWindowsBindingsBuild(t *testing.T) {
	dir := offscreenDir(t)
	unit, err := compiler.Analyse(dir, compiler.Release(target.Windows))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "offscreen.exe")
	if _, err := compiler.Build(context.Background(), unit, binary); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(bytes.ToLower(data), []byte("vulkan-1.dll")) {
		t.Error("the Windows executable does not import vulkan-1.dll")
	}
}
