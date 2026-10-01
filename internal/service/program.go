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
	p.cmd = exec.Command(binPath, "tunnel", "--config", cfgPath, "run")
	p.cmd.Stdout = os.Stdout
	p.cmd.Stderr = os.Stderr
	hideWindow(p.cmd)
	if err := p.cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "cloudflared exited: %v\n", err)
	}
}
