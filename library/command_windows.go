//go:build windows

package library

import (
	"os"
	"os/exec"
	"syscall"
)

func RunCommand(exe string, args ...string) ([]byte, error) {
	instance := exec.Command(exe, args...)
	instance.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	return instance.Output()
}

func RunCmd(prog string, args ...string) error {
	var err error
	cmd := exec.Command(prog, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err = cmd.Start(); err != nil {
		return err
	}

	if err = cmd.Wait(); err != nil {
		return err
	}
	return nil
}
