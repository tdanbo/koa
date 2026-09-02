//go:build windows

package runner

import (
	"fmt"
	"os/exec"
	"syscall"
)

// configureProcessGroup starts the child in a new process group and without a
// console window, so launching from koa never flashes a terminal (PRD §11).
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000, // CREATE_NO_WINDOW
		HideWindow:    true,
	}
}

// terminate ends the process tree. Windows has no SIGTERM equivalent that a
// console-less child reliably observes, so koa asks taskkill to end the tree
// and falls back to a direct kill.
func terminate(cmd *exec.Cmd) error {
	kill := exec.Command("taskkill", "/T", "/PID", fmt.Sprint(cmd.Process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := kill.Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}

// kill forcibly ends the process tree.
func kill(cmd *exec.Cmd) error {
	force := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid))
	force.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := force.Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
