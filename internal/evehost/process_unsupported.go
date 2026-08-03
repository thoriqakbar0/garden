//go:build !darwin && !linux

package evehost

import "os/exec"

func configureProcess(_ *exec.Cmd) {}

func terminateProcess(command *exec.Cmd) {
	if command.Process != nil {
		_ = command.Process.Kill()
	}
}

func killProcess(command *exec.Cmd) {
	if command.Process != nil {
		_ = command.Process.Kill()
	}
}
