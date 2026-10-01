//go:build windows

package process

import (
	"context"
	"os/exec"
	"syscall"
)

// newCmdWindows starts cloudflared with CREATE_NO_WINDOW so no console
// black box pops up on tunnel start.
func newCmdWindows(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd
}
