package hot

import (
	"os"
	"syscall"
)

func checkHost() error {
	return nil
}

func notify(p *os.Process) error {
	return p.Signal(syscall.SIGUSR1)
}
