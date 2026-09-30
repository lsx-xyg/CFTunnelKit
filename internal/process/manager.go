// Package process manages the local cloudflared binary and the per-tunnel
// cloudflared processes (issue #5, slice 03).
//
// Commit 1 of this slice: binary detection + download. Commit 2 adds the
// process lifecycle (Start/Stop/StopAll, log streaming, status events).
package process

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// minBinarySize guards against truncated/corrupt downloads: real
// cloudflared binaries are tens of MB, anything below this is invalid.
const minBinarySize = 1 << 20 // 1MB

// ErrAlreadyRunning is returned when Start is called for a tunnel that is
// already running (issue #5).
var ErrAlreadyRunning = fmt.Errorf("该 Tunnel 已在运行")

// Emitter pushes Wails Events from the manager. The app wires it to
// runtime.EventsEmit; tests may pass nil.
type Emitter func(event string, payload interface{})

// Manager owns the cloudflared binary and (commit 2) the running tunnel
// processes.
type Manager struct {
	binDir  string
	emitter Emitter

	// mu guards the binary download and the running map.
	mu sync.Mutex
}

// NewManager creates a Manager storing the binary under binDir.
func NewManager(binDir string, emitter Emitter) *Manager {
	return &Manager{binDir: binDir, emitter: emitter}
}

// BinPath returns the local cloudflared binary path (.exe on Windows).
func (m *Manager) BinPath() string {
	name := "cloudflared"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(m.binDir, name)
}

// binDirPath returns the manager's storage directory.
func (m *Manager) binDirPath() string {
	return m.binDir
}

// statSize returns the file size or 0 when stat fails.
func statSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}
