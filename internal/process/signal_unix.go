//go:build !windows

package process

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// setProcAttr makes the child a process-group leader so Stop can signal
// the whole process tree (issue #5).
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// signalGroup sends sig to the child's process group (negative pid).
func signalGroup(p *os.Process, sig syscall.Signal) error {
	if p == nil {
		return nil
	}
	return syscall.Kill(-p.Pid, sig)
}

// stopProcess sends SIGTERM to the group, waits grace, then SIGKILLs the
// group (issue #5: macOS/Linux 优雅 → 5s → 强制).
func stopProcess(cmd *exec.Cmd, grace time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = signalGroup(cmd.Process, syscall.SIGTERM)
	time.Sleep(grace)
	_ = signalGroup(cmd.Process, syscall.SIGKILL)
}
