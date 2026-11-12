package peakrss

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	pPID        = 1
	cldTrapped  = 4
	cldStopped  = 5
	exitEvent   = 6
	siCodeOff   = 8
	siginfoSize = 128
)

func Run(newCmd func() *exec.Cmd) (Measured, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cmd := newCmd()
	cmd.SysProcAttr = &syscall.SysProcAttr{Ptrace: true}
	if err := cmd.Start(); err != nil {
		return untraced(newCmd)
	}
	peak := trace(cmd.Process.Pid)
	err := cmd.Wait()
	if peak == 0 {
		peak = maxrss(cmd)
	}
	return Measured{Cmd: cmd, PeakKB: peak}, err
}

func untraced(newCmd func() *exec.Cmd) (Measured, error) {
	cmd := newCmd()
	err := cmd.Run()
	return Measured{Cmd: cmd, PeakKB: maxrss(cmd)}, err
}

func maxrss(cmd *exec.Cmd) int64 {
	if cmd.ProcessState == nil {
		return 0
	}
	usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	return usage.Maxrss
}

func trace(pid int) int64 {
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, syscall.WALL, nil); err != nil || !status.Stopped() {
		return 0
	}
	if err := syscall.PtraceSetOptions(pid, syscall.PTRACE_O_TRACEEXIT); err != nil {
		syscall.PtraceCont(pid, 0)
		return 0
	}
	var peak int64
	signal := 0
	for {
		if err := syscall.PtraceCont(pid, signal); err != nil {
			return peak
		}
		signal = 0
		stopped, err := nextStop(pid, &status)
		if err != nil || !stopped {
			return peak
		}
		switch {
		case status.TrapCause() == exitEvent:
			peak = highWater(pid)
		case status.StopSignal() != syscall.SIGTRAP:
			signal = int(status.StopSignal())
		}
	}
}

func nextStop(pid int, status *syscall.WaitStatus) (bool, error) {
	var info [siginfoSize]byte
	_, _, errno := syscall.Syscall6(syscall.SYS_WAITID, pPID, uintptr(pid), uintptr(unsafe.Pointer(&info[0])), syscall.WEXITED|syscall.WSTOPPED|syscall.WNOWAIT|syscall.WALL, 0, 0)
	if errno != 0 {
		return false, errno
	}
	code := *(*int32)(unsafe.Pointer(&info[siCodeOff]))
	if code != cldTrapped && code != cldStopped {
		return false, nil
	}
	if _, err := syscall.Wait4(pid, status, syscall.WALL, nil); err != nil {
		return false, err
	}
	return status.Stopped(), nil
}

func highWater(pid int) int64 {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	_, rest, found := bytes.Cut(data, []byte("VmHWM:"))
	if !found {
		return 0
	}
	line, _, _ := bytes.Cut(rest, []byte("kB"))
	kb, err := strconv.ParseInt(string(bytes.TrimSpace(line)), 10, 64)
	if err != nil {
		return 0
	}
	return kb
}
