//go:build !windows

package library

import (
	"os"
	"os/exec"
)

func RunCommand(exe string, args ...string) ([]byte, error) {
	instance := exec.Command(exe, args...)

	return instance.Output()
}

func RunCmd(prog string, args ...string) error {
	cmd := exec.Command(prog, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
