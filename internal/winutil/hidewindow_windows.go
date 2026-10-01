//go:build windows

package winutil

import (
	"os/exec"
	"syscall"
)

// HideConsole hides the console window for a child process on Windows.
func HideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
