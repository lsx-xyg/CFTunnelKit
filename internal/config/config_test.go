package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "config.json"))

	cfg := Config{
		Version:     ConfigVersion,
		APIToken:    "tok-abc",
		AccountID:   "acct1",
		AccountName: "Acct One",
		Permissions: Permissions{TunnelEdit: "ok", ZoneRead: "ok", DNSEdit: "unverified"},
		FirstZoneID: "zone1",
		VerifiedAt:  "2026-09-30T12:00:00+08:00",
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.APIToken != "tok-abc" || got.AccountID != "acct1" || got.AccountName != "Acct One" {
		t.Errorf("loaded config mismatch: %+v", got)
	}
	if got.Permissions != cfg.Permissions {
		t.Errorf("permissions = %+v, want %+v", got.Permissions, cfg.Permissions)
	}
	if got.FirstZoneID != "zone1" || got.VerifiedAt != cfg.VerifiedAt {
		t.Errorf("first_zone_id/verified_at mismatch: %+v", got)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(store.Path())
		if err != nil {
			t.Fatalf("stat config: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("config file mode = %o, want 0600", perm)
		}
	}

	// Save with zero Version should default to ConfigVersion.
	if err := store.Save(Config{APIToken: "x"}); err != nil {
		t.Fatalf("Save (zero version): %v", err)
	}
	got, _ = store.Load()
	if got.Version != ConfigVersion {
		t.Errorf("version = %d, want %d", got.Version, ConfigVersion)
	}
}

func TestLoadMissingFile_ReturnsZeroConfig(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "nope", "config.json"))
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg != (Config{}) {
		t.Errorf("cfg = %+v, want zero", cfg)
	}
}

func TestLoadCorruptFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	_, err := store.Load()
	if err == nil {
		t.Fatal("Load: want error for corrupt JSON")
	}
	if !strings.Contains(err.Error(), "配置文件损坏") {
		t.Errorf("error = %q, want to contain 配置文件损坏", err.Error())
	}
}

func TestClear(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "config.json"))
	if err := store.Save(Config{APIToken: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load after Clear: %v", err)
	}
	if cfg != (Config{}) {
		t.Errorf("cfg = %+v, want zero after clear", cfg)
	}
	// Clear on missing file must not error.
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear (missing): %v", err)
	}
}

func TestDefaultPath_UsesHomeDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("HOME-based test is POSIX only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(home, DirName, FileName)
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}
