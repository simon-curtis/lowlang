package amd64

import "syscall"

const (
	ELFEntrySymbol   = "_start"
	PEEntrySymbol    = "start"
	PatchListSymbol  = "rt_patch_list"
	PatchCountSymbol = "rt_patch_count"

	HotBase         = 0x40000000
	HotSize         = 64 << 20
	TableSize       = 4096
	PatchEntryBytes = 16
	PatchSignal     = syscall.Signal(10)

	ImportGroup          = ".idata"
	ImportDescriptors    = ".idata$2"
	ImportNullDescriptor = ".idata$3"
	ImportLookups        = ".idata$4"
	ImportAddresses      = ".idata$5"
	ImportHints          = ".idata$6"
	ImportNames          = ".idata$7"
)
