package config

import (
	"testing"
	"time"
)

func TestLoadFromEnv_MissingIPSetName(t *testing.T) {
	t.Setenv("IPSET_NAME", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error when IPSET_NAME is missing")
	}
}

func TestLoadFromEnv_Valid(t *testing.T) {
	t.Setenv("IPSET_NAME", "lan_hosts")
	t.Setenv("NFLOG_GROUP", "200")
	t.Setenv("CACHE_TTL", "5m")
	t.Setenv("TARGET_CHAN_BUF", "128")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}

	if cfg.IPSetName != "lan_hosts" {
		t.Errorf("IPSetName = %q, want lan_hosts", cfg.IPSetName)
	}
	if cfg.NFLogGroup != 200 {
		t.Errorf("NFLogGroup = %d, want 200", cfg.NFLogGroup)
	}
	if cfg.CacheTTL != 5*time.Minute {
		t.Errorf("CacheTTL = %s, want 5m", cfg.CacheTTL)
	}
	if cfg.TargetChanBuf != 128 {
		t.Errorf("TargetChanBuf = %d, want 128", cfg.TargetChanBuf)
	}
}
