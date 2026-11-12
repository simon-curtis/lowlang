package peakrss

import "os/exec"

type Measured struct {
	Cmd    *exec.Cmd
	PeakKB int64
}
