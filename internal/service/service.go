package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kardianos/service"
)

// Config describes a tunnel's config.yml
type Config struct {
	TunnelID string
	Token    string
	Ingress  []IngressRule
}

type IngressRule struct {
	Hostname string `yaml:"hostname,omitempty"`
	Service  string `yaml:"service"`
}

// Manager wraps kardianos/service
type Manager struct {
	cfgDir string
}

func NewManager(cfgDir string) *Manager {
	return &Manager{cfgDir: cfgDir}
}

func (m *Manager) svcConfig() *service.Config {
	return &service.Config{
		Name:        "cftunnelkit-helper",
		DisplayName: "CFTunnelKit Tunnel Helper",
		Description: "Manages cloudflared tunnel lifecycle for CFTunnelKit",
	}
}

func (m *Manager) newSvc() (service.Service, error) {
	return service.New(&HelperProgram{CfgDir: m.cfgDir}, m.svcConfig())
}

// Install writes config.yml and installs the service
func (m *Manager) Install(cfg Config) error {
	if err := m.WriteConfig(cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	s, err := m.newSvc()
	if err != nil {
		return err
	}
	return s.Install()
}

// Uninstall removes the service
func (m *Manager) Uninstall() error {
	s, err := m.newSvc()
	if err != nil {
		return err
	}
	return s.Uninstall()
}

// Start starts the service
func (m *Manager) Start() error {
	s, err := m.newSvc()
	if err != nil {
		return err
	}
	return s.Start()
}

// Stop stops the service
func (m *Manager) Stop() error {
	s, err := m.newSvc()
	if err != nil {
		return err
	}
	return s.Stop()
}

// Status returns running/stopped/not-installed
func (m *Manager) Status() (string, error) {
	s, err := m.newSvc()
	if err != nil {
		return "not-installed", err
	}
	st, err := s.Status()
	if err != nil || st == service.StatusUnknown {
		return "not-installed", nil
	}
	switch st {
	case service.StatusRunning:
		return "running", nil
	default:
		return "stopped", nil
	}
}

// WriteConfig writes config.yml to the config dir
func (m *Manager) WriteConfig(cfg Config) error {
	if err := os.MkdirAll(m.cfgDir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(m.cfgDir, "config.yml")
	content := fmt.Sprintf("tunnel: %s\ntoken: %s\ningress:\n", cfg.TunnelID, cfg.Token)
	for _, r := range cfg.Ingress {
		if r.Hostname != "" {
			content += fmt.Sprintf("  - hostname: %s\n    service: %s\n", r.Hostname, r.Service)
		} else {
			content += fmt.Sprintf("  - service: %s\n", r.Service)
		}
	}
	content += "  - service: http_status:404\n"
	return os.WriteFile(path, []byte(content), 0o600)
}
