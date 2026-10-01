package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kardianos/service"
)

// HelperProgram implements service.Interface for the standalone helper binary.
type HelperProgram struct {
	CfgDir string
	cmd    *exec.Cmd
}

func (p *HelperProgram) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *HelperProgram) Stop(s service.Service) error {
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	return nil
}

func (p *HelperProgram) run() {
	cfgPath := filepath.Join(p.CfgDir, "config.yml")
	binPath := filepath.Join(p.CfgDir, "bin", "cloudflared")
	logPath := filepath.Join(p.CfgDir, "logs", "service.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o700)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		f = os.Stderr
	}
	p.cmd = exec.Command(binPath, "tunnel", "--config", cfgPath, "run")
	p.cmd.Stdout = f
	p.cmd.Stderr = f
	hideWindow(p.cmd)
	if err := p.cmd.Run(); err != nil {
		fmt.Fprintf(f, "cloudflared exited: %v\n", err)
	}
}
