package main

import (
	"os"
	"path/filepath"

	kservice "github.com/kardianos/service"
	"github.com/lsx-xyg/CFTunnelKit/internal/service"
)

func main() {
	home, _ := os.UserHomeDir()
	cfgDir := filepath.Join(home, ".cftunnelkit")

	svcCfg := &kservice.Config{
		Name:        "cftunnelkit-helper",
		DisplayName: "CFTunnelKit Tunnel Helper",
		Description: "Manages cloudflared tunnel lifecycle for CFTunnelKit",
	}

	prg := &service.HelperProgram{CfgDir: cfgDir}
	s, err := kservice.New(prg, svcCfg)
	if err != nil {
		panic(err)
	}
	if err := s.Run(); err != nil {
		panic(err)
	}
}
