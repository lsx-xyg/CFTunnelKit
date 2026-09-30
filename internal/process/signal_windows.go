//go:build windows

package process

import (
	"os/exec"
	"strconv"
	"time"
)

// setProcAttr is a no-op on Windows: os/exec has no process groups here;
// taskkill handles the process tree (issue #5).
func setProcAttr(cmd *exec.Cmd) {}

// stopProcess tries a graceful `taskkill /PID <pid>` first, then forces
// with `taskkill /F /PID <pid>` after grace (issue #5).
func stopProcess(cmd *exec.Cmd, grace time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	_ = exec.Command("taskkill", "/PID", pid).Run()
	time.Sleep(grace)
	_ = exec.Command("taskkill", "/F", "/PID", pid).Run()
}
