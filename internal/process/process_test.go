package process

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// newTestManager returns a Manager whose commands run `sh -c script`
// (ignoring the cloudflared args), with a pre-placed valid binary so
// EnsureBinary is a no-op, and a buffered event channel.
func newTestManager(t *testing.T, script string, timeout time.Duration) (*Manager, chan map[string]interface{}, func()) {
	t.Helper()
	dir := t.TempDir()
	ch := make(chan map[string]interface{}, 256)
	m := NewManager(dir, func(event string, payload interface{}) {
		ch <- payload.(map[string]interface{})
	})
	m.stopTimeout = timeout
	m.newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", script)
	}
	if err := os.WriteFile(m.BinPath(), make([]byte, 2<<20), 0o755); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		_ = m.StopAll()
		close(ch)
	}
	return m, ch, cleanup
}

// waitEvent reads events until the predicate matches or the timeout hits.
func waitEvent(t *testing.T, ch chan map[string]interface{}, pred func(map[string]interface{}) bool, timeout time.Duration) map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case e := <-ch:
			if pred(e) {
				return e
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	t.Fatalf("timed out waiting for event")
	return nil
}

func TestManager_Start_EmitsRunningAndLogs(t *testing.T) {
	m, ch, cleanup := newTestManager(t, "echo line1; echo line2 >&2; sleep 30", time.Second)
	defer cleanup()

	if err := m.Start(context.Background(), "tun1", "tok1"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := m.RunStates()["tun1"]; got != "running" {
		t.Errorf("RunStates[tun1] = %q, want running", got)
	}

	st := waitEvent(t, ch, func(e map[string]interface{}) bool {
		return e["state"] == "running" && e["tunnel_id"] == "tun1"
	}, 2*time.Second)
	if st["state"] != "running" {
		t.Errorf("status event = %v, want running", st)
	}

	// stdout/stderr stream concurrently; wait until BOTH lines arrive.
	deadline := time.Now().Add(2 * time.Second)
	var seenOut, seenErr bool
	for time.Now().Before(deadline) && (!seenOut || !seenErr) {
		select {
		case e := <-ch:
			if e["stream"] == "stdout" && e["line"] == "line1" {
				seenOut = true
			}
			if e["stream"] == "stderr" && e["line"] == "line2" {
				seenErr = true
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !seenOut {
		t.Error("missing stdout log event line1")
	}
	if !seenErr {
		t.Error("missing stderr log event line2")
	}
}

func TestManager_Start_Duplicate_AlreadyRunning(t *testing.T) {
	m, _, cleanup := newTestManager(t, "sleep 30", time.Second)
	defer cleanup()

	if err := m.Start(context.Background(), "tun1", "tok1"); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	err := m.Start(context.Background(), "tun1", "tok1")
	if err != ErrAlreadyRunning {
		t.Errorf("second Start error = %v, want ErrAlreadyRunning", err)
	}
}

func TestManager_MultipleTunnels_Independent(t *testing.T) {
	m, _, cleanup := newTestManager(t, "sleep 30", time.Second)
	defer cleanup()

	if err := m.Start(context.Background(), "tun1", "tok1"); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background(), "tun2", "tok2"); err != nil {
		t.Fatal(err)
	}
	states := m.RunStates()
	if states["tun1"] != "running" || states["tun2"] != "running" {
		t.Errorf("states = %v, want both running", states)
	}

	if err := m.Stop("tun1"); err != nil {
		t.Fatalf("Stop tun1: %v", err)
	}
	states = m.RunStates()
	if _, ok := states["tun1"]; ok {
		t.Error("tun1 should be stopped")
	}
	if states["tun2"] != "running" {
		t.Error("tun2 should still be running")
	}
	if err := m.Stop("tun2"); err != nil {
		t.Fatalf("Stop tun2: %v", err)
	}
}

func TestManager_Stop_Graceful(t *testing.T) {
	// Trap TERM and exit 0 → graceful path, no force-kill needed.
	m, ch, cleanup := newTestManager(t, "trap 'exit 0' TERM; sleep 30", time.Second)
	defer cleanup()

	if err := m.Start(context.Background(), "tun1", "tok1"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := m.Stop("tun1"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Errorf("graceful stop took %v, should be < 2s", el)
	}
	ev := waitEvent(t, ch, func(e map[string]interface{}) bool {
		return e["state"] == "stopped" && e["tunnel_id"] == "tun1"
	}, 2*time.Second)
	if ev["state"] != "stopped" {
		t.Errorf("status = %v, want stopped", ev["state"])
	}
	if _, ok := m.RunStates()["tun1"]; ok {
		t.Error("tun1 should not be running after stop")
	}
}

func TestManager_Stop_TimeoutForceKill(t *testing.T) {
	// Ignore TERM → Stop must fall through to SIGKILL (超时强杀).
	m, ch, cleanup := newTestManager(t, "trap '' TERM; sleep 30", 300*time.Millisecond)
	defer cleanup()

	if err := m.Start(context.Background(), "tun1", "tok1"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := m.Stop("tun1"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	el := time.Since(start)
	if el < 300*time.Millisecond || el > 4*time.Second {
		t.Errorf("force-kill stop took %v, want ~stopTimeout(300ms)+reap", el)
	}
	ev := waitEvent(t, ch, func(e map[string]interface{}) bool {
		return e["state"] == "stopped" && e["tunnel_id"] == "tun1"
	}, 2*time.Second)
	if ev["state"] != "stopped" {
		t.Errorf("status = %v, want stopped (manager-initiated)", ev["state"])
	}
}

func TestManager_UnexpectedExit_EmitsErrorWithCode(t *testing.T) {
	m, ch, cleanup := newTestManager(t, "echo boom; exit 3", time.Second)
	defer cleanup()

	if err := m.Start(context.Background(), "tun1", "tok1"); err != nil {
		t.Fatal(err)
	}
	ev := waitEvent(t, ch, func(e map[string]interface{}) bool {
		return e["state"] == "error" && e["tunnel_id"] == "tun1"
	}, 3*time.Second)
	if ev["exit_code"] != 3 {
		t.Errorf("exit_code = %v (%T), want 3", ev["exit_code"], ev["exit_code"])
	}
	if _, ok := m.RunStates()["tun1"]; ok {
		t.Error("tun1 should not be running after unexpected exit")
	}
}

func TestManager_Logs_ANSIStripped(t *testing.T) {
	m, ch, cleanup := newTestManager(t, `printf '\033[31mred\033[0m\n'; sleep 30`, time.Second)
	defer cleanup()

	if err := m.Start(context.Background(), "tun1", "tok1"); err != nil {
		t.Fatal(err)
	}
	ev := waitEvent(t, ch, func(e map[string]interface{}) bool {
		return e["stream"] == "stdout" && strings.Contains(e["line"].(string), "red")
	}, 2*time.Second)
	if ev["line"] != "red" {
		t.Errorf("line = %q, want ANSI-stripped 'red'", ev["line"])
	}
}

func TestManager_StopAll(t *testing.T) {
	m, _, cleanup := newTestManager(t, "sleep 30", time.Second)
	defer cleanup()

	if err := m.Start(context.Background(), "tun1", "tok1"); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background(), "tun2", "tok2"); err != nil {
		t.Fatal(err)
	}
	if err := m.StopAll(); err != nil {
		t.Fatalf("StopAll: %v", err)
	}
	if got := m.RunStates(); len(got) != 0 {
		t.Errorf("RunStates after StopAll = %v, want empty", got)
	}
}

func TestManager_Stop_NotRunning_Error(t *testing.T) {
	m, _, cleanup := newTestManager(t, "sleep 30", time.Second)
	defer cleanup()
	if err := m.Stop("nope"); err == nil {
		t.Error("Stop of non-running tunnel should error")
	}
}
