//go:build windows

package process

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// setProcAttr is a no-op on Windows: os/exec has no process groups here;
// taskkill handles the process tree (issue #5).
func setProcAttr(cmd *exec.Cmd) {}

// noWindow hides the console window for child commands on Windows.
var noWindow = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW

// stopProcess tries a graceful `taskkill /PID <pid>` first, then forces
// with `taskkill /F /PID <pid>` after grace (issue #5).
func stopProcess(cmd *exec.Cmd, grace time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	graceKill := exec.Command("taskkill", "/PID", pid)
	graceKill.SysProcAttr = noWindow
	_ = graceKill.Run()
	time.Sleep(grace)
	forceKill := exec.Command("taskkill", "/F", "/PID", pid)
	forceKill.SysProcAttr = noWindow
	_ = forceKill.Run()
}
