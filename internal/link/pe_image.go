package link

import (
	"debug/pe"
)

const (
	peLinkerMajor     uint8  = 14
	peVersionMajor    uint16 = 6
	pePlus64Magic     uint16 = 0x20b
	peDataDirectories        = len(pe.OptionalHeader64{}.DataDirectory)
)

var (
	peSignatureSize      = structSize(peSignature)
	peFileHeaderSize     = structSize(pe.FileHeader{})
	peOptionalHeaderSize = structSize(pe.OptionalHeader64{})
	peSectionHeaderSize  = structSize(pe.SectionHeader32{})
)

const (
	peDllCharacteristics = pe.IMAGE_DLLCHARACTERISTICS_HIGH_ENTROPY_VA |
		pe.IMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE |
		pe.IMAGE_DLLCHARACTERISTICS_NX_COMPAT |
		pe.IMAGE_DLLCHARACTERISTICS_TERMINAL_SERVER_AWARE
	peFileCharacteristics = pe.IMAGE_FILE_EXECUTABLE_IMAGE | pe.IMAGE_FILE_LARGE_ADDRESS_AWARE
)

const (
	peDirImport    = pe.IMAGE_DIRECTORY_ENTRY_IMPORT
	peDirBaseReloc = pe.IMAGE_DIRECTORY_ENTRY_BASERELOC
	peDirDebug     = pe.IMAGE_DIRECTORY_ENTRY_DEBUG
	peDirIAT       = pe.IMAGE_DIRECTORY_ENTRY_IAT
)

var peSignature = [4]byte{'P', 'E', 0, 0}

var peDOSStub = [120]byte{
	0x4d, 0x5a, 0x78, 0x00, 0x01, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x78, 0x00, 0x00, 0x00, 0x0e, 0x1f, 0xba, 0x0e, 0x00, 0xb4, 0x09, 0xcd,
	0x21, 0xb8, 0x01, 0x4c, 0xcd, 0x21, 0x54, 0x68, 0x69, 0x73, 0x20, 0x70,
	0x72, 0x6f, 0x67, 0x72, 0x61, 0x6d, 0x20, 0x63, 0x61, 0x6e, 0x6e, 0x6f,
	0x74, 0x20, 0x62, 0x65, 0x20, 0x72, 0x75, 0x6e, 0x20, 0x69, 0x6e, 0x20,
	0x44, 0x4f, 0x53, 0x20, 0x6d, 0x6f, 0x64, 0x65, 0x2e, 0x24, 0x00, 0x00,
}

func (l *peLinker) sizes() (code, initialized, uninitialized uint32, baseOfCode uint32) {
	for _, s := range l.sections {
		switch {
		case s.isCode():
			if code == 0 {
				baseOfCode = uint32(s.rva)
			}
			code += uint32(s.rawSize)
		case s.chars&pe.IMAGE_SCN_CNT_INITIALIZED_DATA != 0:
			initialized += uint32(s.rawSize)
		default:
			uninitialized += uint32(alignUp(s.virtualSize, peFileAlign))
		}
	}
	return code, initialized, uninitialized, baseOfCode
}

func (l *peLinker) dataDirectories() [peDataDirectories]pe.DataDirectory {
	var dirs [peDataDirectories]pe.DataDirectory
	dirs[peDirDebug] = pe.DataDirectory{VirtualAddress: uint32(l.debug.rva()), Size: uint32(l.debug.size)}
	if span, ok := l.prefixSpan(importPrefix); ok {
		dirs[peDirImport] = span
	}
	if span, ok := l.prefixSpan(iatPrefix); ok {
		dirs[peDirIAT] = span
	}
	if len(l.sites) > 0 {
		last := l.sections[len(l.sections)-1]
		dirs[peDirBaseReloc] = pe.DataDirectory{VirtualAddress: uint32(last.rva), Size: uint32(last.virtualSize)}
	}
	return dirs
}

func (l *peLinker) writeHeaders(image []byte, entry uint64, timestamp uint32) error {
	code, initialized, uninitialized, baseOfCode := l.sizes()
	w := &structWriter{}
	w.add(peDOSStub)
	w.add(peSignature)
	w.add(pe.FileHeader{
		Machine:              pe.IMAGE_FILE_MACHINE_AMD64,
		NumberOfSections:     uint16(len(l.sections)),
		SizeOfOptionalHeader: uint16(peOptionalHeaderSize),
		TimeDateStamp:        timestamp,
		Characteristics:      peFileCharacteristics,
	})
	w.add(pe.OptionalHeader64{
		Magic:                       pePlus64Magic,
		MajorLinkerVersion:          peLinkerMajor,
		SizeOfCode:                  code,
		SizeOfInitializedData:       initialized,
		SizeOfUninitializedData:     uninitialized,
		AddressOfEntryPoint:         uint32(entry),
		BaseOfCode:                  baseOfCode,
		ImageBase:                   peImageBase,
		SectionAlignment:            uint32(peSectionAlign),
		FileAlignment:               uint32(peFileAlign),
		MajorOperatingSystemVersion: peVersionMajor,
		MajorSubsystemVersion:       peVersionMajor,
		SizeOfImage:                 uint32(l.imageSize),
		SizeOfHeaders:               uint32(l.headerSize),
		Subsystem:                   pe.IMAGE_SUBSYSTEM_WINDOWS_CUI,
		DllCharacteristics:          peDllCharacteristics,
		SizeOfStackReserve:          peStackReserve,
		SizeOfStackCommit:           peStackCommit,
		SizeOfHeapReserve:           peHeapReserve,
		SizeOfHeapCommit:            peHeapCommit,
		NumberOfRvaAndSizes:         uint32(peDataDirectories),
		DataDirectory:               l.dataDirectories(),
	})
	for _, s := range l.sections {
		w.add(s.header())
	}
	encoded, err := w.bytes()
	if err != nil {
		return err
	}
	copy(image, encoded)
	return nil
}

func (s *peSection) header() pe.SectionHeader32 {
	header := pe.SectionHeader32{
		VirtualSize:      uint32(s.virtualSize),
		VirtualAddress:   uint32(s.rva),
		SizeOfRawData:    uint32(s.rawSize),
		PointerToRawData: uint32(s.rawOffset),
		Characteristics:  s.chars,
	}
	copy(header.Name[:], s.name)
	return header
}

func (l *peLinker) stamp(image []byte, entry uint64) error {
	timestamp := uint32(xxh3Long(image))
	if err := l.writeHeaders(image, entry, timestamp); err != nil {
		return err
	}
	debug, err := l.debugEntry(timestamp)
	if err != nil {
		return err
	}
	copy(image[l.debug.fileOffset():], debug)
	return nil
}
