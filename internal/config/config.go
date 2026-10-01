// Package config persists the app's local configuration as a JSON file in
// the user's home directory (~/.cftunnelkit/config.json on POSIX,
// %USERPROFILE%\.cftunnelkit\config.json on Windows; os.UserHomeDir() unifies
// both). The file is written with 0600 permissions on POSIX.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigVersion is the current schema version written to config.json.
const ConfigVersion = 1

// DirName is the config directory name under the user's home directory.
const DirName = ".cftunnelkit"

// FileName is the config file name.
const FileName = "config.json"

// Permissions mirrors cloudflare.Permissions as persisted strings so the
// config file keeps the plain "ok" / "missing" / "unverified" vocabulary.
type Permissions struct {
	TunnelEdit string `json:"tunnel_edit"`
	ZoneRead   string `json:"zone_read"`
	DNSEdit    string `json:"dns_edit"`
}

// Config is the on-disk schema of config.json.
type Config struct {
	Version     int         `json:"version"`
	APIToken    string      `json:"api_token"`
	AccountID   string      `json:"account_id"`
	AccountName string      `json:"account_name"`
	Permissions Permissions `json:"permissions"`
	FirstZoneID string      `json:"first_zone_id"`
	// VerifiedAt is the RFC3339 timestamp of the last successful verification.
	VerifiedAt string `json:"verified_at"`
	LastRunning []string `json:"last_running,omitempty"`
}

// Store reads and writes Config to a single file.
type Store struct {
	path string
}

// NewStore returns a Store bound to the given config file path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path returns the config file path this store is bound to.
func (s *Store) Path() string { return s.path }

// DefaultPath returns the default config file location:
// <home>/.cftunnelkit/config.json (os.UserHomeDir() unifies POSIX and Windows).
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法获取用户主目录: %w", err)
	}
	return filepath.Join(home, DirName, FileName), nil
}

// DefaultBinDir returns the cloudflared binary directory under the same
// home-based app dir (~/.cftunnelkit/bin; issue #5, slice 03).
func DefaultBinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法获取用户主目录: %w", err)
	}
	return filepath.Join(home, DirName, "bin"), nil
}

// DefaultLogPath returns the rolling cloudflared log file under the
// home-based app dir (~/.cftunnelkit/logs/cloudflared.log; issue #9,
// slice 07b: MaxSize 1MB, 3 backups, no compression).
func DefaultLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法获取用户主目录: %w", err)
	}
	return filepath.Join(home, DirName, "logs", "cloudflared.log"), nil
}

// Load reads the config file. A missing file yields a zero Config with no
// error (treated as unauthenticated). A corrupt file yields an error, which
// callers must treat as unauthenticated as well (no crash).
func (s *Store) Load() (Config, error) {
	var cfg Config
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("配置文件损坏: %w", err)
	}
	return cfg, nil
}

// Save writes the config atomically-ish with 0600 permissions
// (POSIX; the mode is ignored on Windows).
func (s *Store) Save(cfg Config) error {
	if cfg.Version == 0 {
		cfg.Version = ConfigVersion
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	// Write to a temp file and rename to avoid a partially-written config.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	return os.Rename(tmp, s.path)
}

// Clear removes the config file (used when the token becomes invalid).
func (s *Store) Clear() error {
	err := os.Remove(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
