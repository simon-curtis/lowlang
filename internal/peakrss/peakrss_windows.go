package peakrss

import (
	"os/exec"
	"syscall"
	"unsafe"
)

const processQueryLimitedInformation = 0x1000

var getProcessMemoryInfo = syscall.NewLazyDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

type processMemoryCounters struct {
	size               uint32
	pageFaultCount     uint32
	peakWorkingSetSize uintptr
	_                  [7]uintptr
}

func Run(newCmd func() *exec.Cmd) (Measured, error) {
	cmd := newCmd()
	if err := cmd.Start(); err != nil {
		return Measured{Cmd: cmd}, err
	}
	process, openErr := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(cmd.Process.Pid))
	err := cmd.Wait()
	if openErr != nil {
		return Measured{Cmd: cmd}, err
	}
	defer syscall.CloseHandle(process)
	return Measured{Cmd: cmd, PeakKB: peakWorkingSet(process)}, err
}

func peakWorkingSet(process syscall.Handle) int64 {
	if getProcessMemoryInfo.Find() != nil {
		return 0
	}
	var counters processMemoryCounters
	counters.size = uint32(unsafe.Sizeof(counters))
	ok, _, _ := getProcessMemoryInfo.Call(uintptr(process), uintptr(unsafe.Pointer(&counters)), uintptr(counters.size))
	if ok == 0 {
		return 0
	}
	return int64(counters.peakWorkingSetSize / 1024)
}
