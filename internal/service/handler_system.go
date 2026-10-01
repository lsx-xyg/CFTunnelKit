package service

import (
	"context"
	"os"
	"os/exec"
	"runtime"

	"github.com/lsx-xyg/CFTunnelKit/internal/applog"
	"github.com/lsx-xyg/CFTunnelKit/internal/version"
)

// SystemHandler owns log-directory and operation-log bindings.
type SystemHandler struct {
	ctx context.Context
}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

func (h *SystemHandler) SetContext(ctx context.Context) {
	h.ctx = ctx
}

// LogDir returns the absolute path of the directory containing app.log and
// cloudflared.log.
func (h *SystemHandler) LogDir() string {
	return applog.Dir()
}

// OpenLogDir opens the logs folder in the system file manager.
func (h *SystemHandler) OpenLogDir() {
	d := applog.Dir()
	_ = os.MkdirAll(d, 0o700)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("explorer", d)
	} else if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", d)
	} else {
		cmd = exec.Command("xdg-open", d)
	}
	_ = cmd.Start()
}

// WriteOpLog appends an operation record to operation.log.
func (h *SystemHandler) WriteOpLog(action, result string) {
	applog.OpLog(action, result)
}

// GetVersion returns the build version (injected via ldflags).
func (h *SystemHandler) GetVersion() string {
	return version.Version
}
