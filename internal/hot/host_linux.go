package hot

import (
	"os"

	"lowlang/internal/amd64"
)

func checkHost() error {
	return nil
}

func notify(p *os.Process) error {
	return p.Signal(amd64.PatchSignal)
}
