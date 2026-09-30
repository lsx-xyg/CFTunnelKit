package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// RunState is the per-tunnel process state pushed to the frontend.
type RunState string

const (
	StateRunning RunState = "running"
	StateStopped RunState = "stopped"
	StateError   RunState = "error"
)

// ProcessManager is the seam for tunnel process control (issue #5). The
// app consumes this interface; tests can inject a mock.
type ProcessManager interface {
	EnsureBinary(ctx context.Context) error
	Start(ctx context.Context, tunnelID, token string) error
	Stop(tunnelID string) error
	StopAll() error
	RunStates() map[string]string
}

var _ ProcessManager = (*Manager)(nil)

// defaultStopTimeout is how long Stop waits for graceful exit before
// force-killing (issue #5: SIGTERM → 5s → SIGKILL). Tests override it.
const defaultStopTimeout = 5 * time.Second

// ansiRe strips ANSI color sequences (e.g. \x1b[31m) before pushing logs.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// cloudflaredRe matches the zap log level token cloudflared emits
// (e.g. "2026-09-30T09:30:30Z INF ..."). All cloudflared logs go to stderr,
// so coloring by stream mislabels every INFO line as an error; we parse the
// level token out of the line instead.
var cloudflaredLevelRe = regexp.MustCompile(`\b(INF|WRN|ERR|DBG)\b`)

// logLevel maps a cloudflared log line to INFO/WARN/ERROR/DEBUG.
func logLevel(line string) string {
	m := cloudflaredLevelRe.FindStringSubmatch(line)
	if m == nil {
		return "INFO"
	}
	switch m[1] {
	case "ERR":
		return "ERROR"
	case "WRN":
		return "WARN"
	case "DBG":
		return "DEBUG"
	default:
		return "INFO"
	}
}

// tunnelProc is one running cloudflared process.
type tunnelProc struct {
	cmd     *exec.Cmd
	exited  chan struct{} // closed once cmd.Wait returned
	stopped atomic.Bool   // true when Stop/StopAll initiated the shutdown
}

// Start launches `cloudflared tunnel run --token <token>` for the tunnel.
// The tunnel slot is reserved before the binary download, so a second Start
// of the same tunnel is rejected even while the first is still downloading.
// The "running" status is pushed right after Start() succeeds, not after
// the cloudflared connection is ready (issue #5).
func (m *Manager) Start(ctx context.Context, tunnelID, token string) error {
	if strings.TrimSpace(tunnelID) == "" {
		return fmt.Errorf("缺少 Tunnel ID")
	}
	m.mu.Lock()
	if _, ok := m.running[tunnelID]; ok {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}
	m.running[tunnelID] = nil // reserve the slot
	m.mu.Unlock()

	released := false
	release := func() {
		if released {
			return
		}
		released = true
		m.mu.Lock()
		if m.running[tunnelID] == nil {
			delete(m.running, tunnelID)
		}
		m.mu.Unlock()
	}
	defer release()

	if err := m.EnsureBinary(ctx); err != nil {
		return err
	}

	cmd := m.newCmd(ctx, m.BinPath(), "tunnel", "run", "--token", token)
	setProcAttr(cmd) // process-group leader on unix

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建 stderr 管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 cloudflared 失败: %w", err)
	}

	p := &tunnelProc{cmd: cmd, exited: make(chan struct{})}
	m.mu.Lock()
	m.running[tunnelID] = p
	m.mu.Unlock()

	go m.scanStream(stdout, tunnelID, "stdout")
	go m.scanStream(stderr, tunnelID, "stderr")
	go m.watch(p, tunnelID)

	m.emit("cloudflared:status", map[string]interface{}{
		"tunnel_id": tunnelID, "state": string(StateRunning),
	})
	return nil
}

// watch reaps the process and pushes the terminal status. Unexpected exits
// push status=error with the exit code; Stop-initiated exits push
// status=stopped (issue #5: no auto-restart).
func (m *Manager) watch(p *tunnelProc, tunnelID string) {
	err := p.cmd.Wait()
	close(p.exited)

	m.mu.Lock()
	if m.running[tunnelID] == p {
		delete(m.running, tunnelID)
	}
	m.mu.Unlock()

	if p.stopped.Load() {
		m.emit("cloudflared:status", map[string]interface{}{
			"tunnel_id": tunnelID, "state": string(StateStopped),
		})
		return
	}
	m.emit("cloudflared:status", map[string]interface{}{
		"tunnel_id": tunnelID, "state": string(StateError), "exit_code": exitCode(err),
	})
}

// scanStream reads a command stream line by line and pushes
// cloudflared:log events {timestamp, stream, level, line, tunnel_id} with
// ANSI sequences stripped. The same stripped line is appended to the rolling
// log file (slice 07b) when a writer is installed.
func (m *Manager) scanStream(r io.Reader, tunnelID, stream string) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := ansiRe.ReplaceAllString(sc.Text(), "")
		if m.logWriter != nil {
			_, _ = io.WriteString(m.logWriter, line+"\n")
		}
		m.emit("cloudflared:log", map[string]interface{}{
			"timestamp": time.Now().UnixMilli(),
			"stream":    stream,
			"level":     logLevel(line),
			"line":      line,
			"tunnel_id": tunnelID,
		})
	}
}

// Stop gracefully stops one tunnel: platform stop signal, then a force-kill
// after stopTimeout (issue #5). The caller waits for the process to exit.
func (m *Manager) Stop(tunnelID string) error {
	m.mu.Lock()
	p, ok := m.running[tunnelID]
	m.mu.Unlock()
	if !ok || p == nil {
		return fmt.Errorf("该 Tunnel 未在运行")
	}
	p.stopped.Store(true)
	stopProcess(p.cmd, m.stopTimeout)

	select {
	case <-p.exited:
		return nil
	case <-time.After(m.stopTimeout + 2*time.Second):
		return fmt.Errorf("停止超时")
	}
}

// StopAll stops every running tunnel (app shutdown path).
func (m *Manager) StopAll() error {
	m.mu.Lock()
	ids := make([]string, 0, len(m.running))
	for id := range m.running {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	var firstErr error
	for _, id := range ids {
		if err := m.Stop(id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// RunStates returns a snapshot of tunnels currently running.
func (m *Manager) RunStates() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]string, len(m.running))
	for id, p := range m.running {
		if p != nil {
			out[id] = string(StateRunning)
		}
	}
	return out
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
