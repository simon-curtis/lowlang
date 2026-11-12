package link_test

import (
	"encoding/binary"
	"strings"
	"testing"

	"lowlang/internal/amd64"
	"lowlang/internal/asm"
	"lowlang/internal/link"
)

const patchBase uint64 = 0x40000000

func patchObject(fixups []asm.Fixup) *asm.Object {
	text := &asm.Section{Name: ".text", Flags: asm.FlagAlloc | asm.FlagExec, Align: 32, Data: make([]byte, 32), Fixups: fixups}
	rodata := &asm.Section{Name: ".rodata", Flags: asm.FlagAlloc, Align: 8, Data: []byte{1, 2, 3, 4, 5, 6, 7, 8}}
	return &asm.Object{
		Sections: []*asm.Section{text, rodata},
		Symbols: []asm.Symbol{
			{Name: "fn_patched", Section: 0, Global: true},
			{Name: "message", Section: 1, Value: 4},
			{Name: "constant", Section: asm.SectionAbsolute, Value: 0x1234},
		},
	}
}

func TestPatchResolvesAgainstTheRunningImage(t *testing.T) {
	running := map[string]uint64{"rt_entry": 0x401000}
	obj := patchObject([]asm.Fixup{
		{Offset: 1, Kind: asm.PLT32, Symbol: "rt_entry", Addend: -4},
		{Offset: 8, Kind: asm.PC32, Symbol: "message", Addend: -4},
		{Offset: 16, Kind: asm.Abs64, Symbol: "constant"},
	})
	patch, err := link.Patch(obj, patchBase+5, running)
	if err != nil {
		t.Fatal(err)
	}
	if patch.Address != patchBase+32 {
		t.Fatalf("patch at %#x, want the next 32-byte boundary %#x", patch.Address, patchBase+32)
	}
	if got := patch.Symbols["fn_patched"]; got != patch.Address {
		t.Errorf("fn_patched at %#x, want %#x", got, patch.Address)
	}
	wantMessage := patch.Address + 32 + 4
	if got := patch.Symbols["message"]; got != wantMessage {
		t.Errorf("message at %#x, want %#x", got, wantMessage)
	}
	call := int32(binary.LittleEndian.Uint32(patch.Code[1:]))
	if want := int32(0x401000 - 4 - int64(patch.Address+1)); call != want {
		t.Errorf("call displacement %d, want %d", call, want)
	}
	reference := int32(binary.LittleEndian.Uint32(patch.Code[8:]))
	if want := int32(int64(wantMessage) - 4 - int64(patch.Address+8)); reference != want {
		t.Errorf("rodata displacement %d, want %d", reference, want)
	}
	if got := binary.LittleEndian.Uint64(patch.Code[16:]); got != 0x1234 {
		t.Errorf("absolute value %#x, want 0x1234", got)
	}
}

func TestPatchRejectsWhatItCannotResolve(t *testing.T) {
	cases := []struct {
		name   string
		fixups []asm.Fixup
		want   string
	}{
		{"undefined symbol", []asm.Fixup{{Offset: 1, Kind: asm.PLT32, Symbol: "missing"}}, "undefined symbol missing"},
		{"got relocation", []asm.Fixup{{Offset: 1, Kind: asm.GOTPCREL, Symbol: "rt_entry"}}, "not supported"},
		{"out of range", []asm.Fixup{{Offset: 1, Kind: asm.PC32, Symbol: "far"}}, "does not fit"},
	}
	running := map[string]uint64{"rt_entry": 0x401000, "far": 1 << 40}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := link.Patch(patchObject(c.fixups), patchBase, running)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want %q", err, c.want)
			}
		})
	}
}

func TestPatchRejectsWritableSections(t *testing.T) {
	obj := patchObject(nil)
	obj.Sections = append(obj.Sections, &asm.Section{Name: ".data", Flags: asm.FlagAlloc | asm.FlagWrite, Align: 8, Data: make([]byte, 8)})
	if _, err := link.Patch(obj, patchBase, nil); err == nil {
		t.Fatal("patched a program that owns writable data")
	}
}

func TestLinkReturnsTheResolvedSymbols(t *testing.T) {
	obj := fixtureObject(fixtureOptions{})
	image, err := link.Static(obj)
	if err != nil {
		t.Fatal(err)
	}
	if symbols := image.Symbols; symbols[amd64.ELFEntrySymbol] == 0 || symbols["data_label"] == 0 {
		t.Errorf("entry and data labels missing from %v", symbols)
	}
}
