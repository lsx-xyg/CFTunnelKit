package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/natefinch/lumberjack.v2"
)

// TestRollingLog_RotatesAt1MB_Keeps3Backups verifies the issue #9
// contract directly against lumberjack with the production parameters
// (MaxSize=1MB, MaxBackups=3, Compress=false).
func TestRollingLog_RotatesAt1MB_Keeps3Backups(t *testing.T) {
	dir := t.TempDir()
	lj := &lumberjack.Logger{
		Filename:   filepath.Join(dir, "cloudflared.log"),
		MaxSize:    1,
		MaxBackups: 3,
		MaxAge:     0,
		Compress:   false,
	}
	line := []byte(strings.Repeat("a", 1024) + "\n") // 1KB per write
	for i := 0; i < 3600; i++ {                      // ~3.6MB total
		if _, err := lj.Write(line); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := lj.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var backups int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "cloudflared.log.") {
			backups++
		}
	}
	if backups > 3 {
		t.Errorf("backups = %d, want <= 3 (MaxBackups=3)", backups)
	}
	if _, err := os.Stat(filepath.Join(dir, "cloudflared.log")); err != nil {
		t.Errorf("current log file missing: %v", err)
	}
	// Compress=false: backups must be plain files, not .gz
	for _, e := range entries {
		if strings.Contains(e.Name(), ".log.") && strings.HasSuffix(e.Name(), ".gz") {
			t.Errorf("backup %s is compressed, want plain text (Compress=false)", e.Name())
		}
	}
}

// TestManager_ScanStream_WritesToLogFile verifies that every line pushed to
// the frontend is also appended to the rolling log (ANSI-stripped).
func TestManager_ScanStream_WritesToLogFile(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, nil)
	lj := &lumberjack.Logger{Filename: filepath.Join(dir, "cloudflared.log"), MaxSize: 1, MaxBackups: 3}
	m.SetLogWriter(lj)

	events := 0
	m.emitter = func(event string, payload interface{}) {
		if event == "cloudflared:log" {
			events++
		}
	}
	m.scanStream(strings.NewReader("first line\n\x1b[31msecond line\x1b[0m\n"), "tun1", "stdout")

	if events != 2 {
		t.Errorf("events = %d, want 2", events)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "cloudflared.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, "first line") || !strings.Contains(s, "second line") {
		t.Errorf("log content = %q, want both lines", s)
	}
	if strings.Contains(s, "\x1b") {
		t.Errorf("log contains ANSI escapes, want stripped")
	}
}

func TestLogLevelParsing(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"2026-09-30T09:30:30Z INF Starting tunnel", "INFO"},
		{"2026-09-30T09:30:30Z ERR failed to dial", "ERROR"},
		{"2026-09-30T09:30:30Z WRN low memory", "WARN"},
		{"2026-09-30T09:30:30Z DBG debug thing", "DEBUG"},
		{"no level token here", "INFO"},
	}
	for _, c := range cases {
		if got := logLevel(c.line); got != c.want {
			t.Errorf("logLevel(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}
