// Package process manages the local cloudflared binary and the per-tunnel
// cloudflared processes (issue #5, slice 03).
//
// Commit 1 of this slice: binary detection + download. Commit 2 adds the
// process lifecycle (Start/Stop/StopAll, log streaming, status events).
package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
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

// Manager owns the cloudflared binary and the running tunnel processes.
type Manager struct {
	binDir  string
	emitter Emitter
	logWriter io.Writer // optional rolling file (slice 07b); nil disables

	mu          sync.Mutex
	running     map[string]*tunnelProc
	stopTimeout time.Duration
	newCmd      func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewManager creates a Manager storing the binary under binDir.
func NewManager(binDir string, emitter Emitter) *Manager {
	m := &Manager{
		binDir:      binDir,
		emitter:     emitter,
		running:     map[string]*tunnelProc{},
		stopTimeout: defaultStopTimeout,
	}
	m.newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command(name, args...)
	}
	return m
}

// SetLogWriter installs the rolling log writer (issue #9). Every stripped
// log line pushed to the frontend is also written here with a trailing
// newline. The writer must be safe for concurrent use (lumberjack is).
func (m *Manager) SetLogWriter(w io.Writer) {
	m.logWriter = w
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
