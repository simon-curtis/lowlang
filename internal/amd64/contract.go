package amd64

const (
	ELFEntrySymbol   = "_start"
	PEEntrySymbol    = "start"
	PatchListSymbol  = "rt_patch_list"
	PatchCountSymbol = "rt_patch_count"

	ImportGroup          = ".idata"
	ImportDescriptors    = ".idata$2"
	ImportNullDescriptor = ".idata$3"
	ImportLookups        = ".idata$4"
	ImportAddresses      = ".idata$5"
	ImportHints          = ".idata$6"
	ImportNames          = ".idata$7"
)
