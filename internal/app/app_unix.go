//go:build !windows

package app

import "os/exec"

// platformRevealCommand opens dir in the Linux desktop's file manager.
func platformRevealCommand(dir string) *exec.Cmd {
	return exec.Command("xdg-open", dir)
}
