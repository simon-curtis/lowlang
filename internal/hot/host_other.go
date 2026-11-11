//go:build !linux

package hot

import (
	"errors"
	"os"
)

var errHost = errors.New("hot reload requires a Linux host")

func checkHost() error {
	return errHost
}

func notify(*os.Process) error {
	return errHost
}
