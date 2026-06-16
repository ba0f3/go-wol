package service

import (
	"strings"
	"testing"
	"time"

	"github.com/tui/go-wol/config"
)

func TestRenderUnit(t *testing.T) {
	cfg := config.Config{
		NFLogGroup:    100,
		IPSetName:     "lan_hosts",
		CacheTTL:      2 * time.Minute,
		TargetChanBuf: 64,
	}

	unit := RenderUnit(cfg, "/usr/local/bin/go-wol")

	checks := []string{
		"Description=Tailscale Wake-on-LAN daemon",
		"After=network-online.target tailscaled.service",
		"ExecStart=/usr/local/bin/go-wol",
		"Environment=IPSET_NAME=lan_hosts",
		"Environment=NFLOG_GROUP=100",
		"Environment=CACHE_TTL=2m0s",
		"Environment=TARGET_CHAN_BUF=64",
		"Restart=on-failure",
		"WantedBy=multi-user.target",
	}
	for _, want := range checks {
		if !strings.Contains(unit, want) {
			t.Errorf("unit missing %q\n%s", want, unit)
		}
	}
}
