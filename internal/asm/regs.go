package asm

import "fmt"

type RegClass uint8

const (
	ClassNone RegClass = iota
	ClassGP8
	ClassGP8High
	ClassGP16
	ClassGP32
	ClassGP64
	ClassXMM
	ClassYMM
	ClassSegment
	ClassIP
)

type Reg uint8

const RegNone Reg = 0

type regInfo struct {
	name  string
	class RegClass
	num   uint8
}

var regInfos = buildRegInfos()

var regByName = buildRegByName()

var regRIP = lookupReg("rip")

var gpNames = [16][4]string{
	{"al", "ax", "eax", "rax"},
	{"cl", "cx", "ecx", "rcx"},
	{"dl", "dx", "edx", "rdx"},
	{"bl", "bx", "ebx", "rbx"},
	{"spl", "sp", "esp", "rsp"},
	{"bpl", "bp", "ebp", "rbp"},
	{"sil", "si", "esi", "rsi"},
	{"dil", "di", "edi", "rdi"},
}

var highByteNames = [4]string{"ah", "ch", "dh", "bh"}

var segmentNames = [6]string{"es", "cs", "ss", "ds", "fs", "gs"}

func buildRegInfos() []regInfo {
	infos := []regInfo{{}}
	classes := [4]RegClass{ClassGP8, ClassGP16, ClassGP32, ClassGP64}
	for width, class := range classes {
		for n := range 16 {
			info := newGPInfo(n, width, class)
			infos = append(infos, info)
		}
	}
	for i, name := range highByteNames {
		infos = append(infos, regInfo{name: name, class: ClassGP8High, num: uint8(4 + i)})
	}
	for n := range 16 {
		xmm := fmt.Sprintf("xmm%d", n)
		infos = append(infos, regInfo{name: xmm, class: ClassXMM, num: uint8(n)})
	}
	for n := range 16 {
		ymm := fmt.Sprintf("ymm%d", n)
		infos = append(infos, regInfo{name: ymm, class: ClassYMM, num: uint8(n)})
	}
	for i, name := range segmentNames {
		infos = append(infos, regInfo{name: name, class: ClassSegment, num: uint8(i)})
	}
	return append(infos, regInfo{name: "rip", class: ClassIP})
}

func newGPInfo(n, width int, class RegClass) regInfo {
	if n < 8 {
		return regInfo{name: gpNames[n][width], class: class, num: uint8(n)}
	}
	suffix := [4]string{"b", "w", "d", ""}[width]
	return regInfo{name: fmt.Sprintf("r%d%s", n, suffix), class: class, num: uint8(n)}
}

func buildRegByName() map[string]Reg {
	byName := make(map[string]Reg, len(regInfos))
	for i := 1; i < len(regInfos); i++ {
		byName[regInfos[i].name] = Reg(i)
	}
	return byName
}

func (r Reg) Class() RegClass {
	return regInfos[r].class
}

func (r Reg) Num() uint8 {
	return regInfos[r].num
}

func (r Reg) String() string {
	return regInfos[r].name
}

func lookupReg(name string) Reg {
	return regByName[name]
}
