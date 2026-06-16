package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// Config holds daemon configuration loaded from environment variables.
type Config struct {
	NFLogGroup    uint16
	IPSetName     string
	CacheTTL      time.Duration
	TargetChanBuf int
}

// LoadFromEnv reads configuration from environment variables.
func LoadFromEnv() (Config, error) {
	cfg := Config{
		NFLogGroup:    100,
		IPSetName:     "macbinding",
		CacheTTL:      2 * time.Minute,
		TargetChanBuf: 64,
	}

	if v := os.Getenv("NFLOG_GROUP"); v != "" {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Config{}, fmt.Errorf("invalid NFLOG_GROUP %q: %w", v, err)
		}
		cfg.NFLogGroup = uint16(n)
	}

	cfg.IPSetName = os.Getenv("IPSET_NAME")
	if cfg.IPSetName == "" {
		return Config{}, fmt.Errorf("IPSET_NAME environment variable is required")
	}

	if v := os.Getenv("CACHE_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CACHE_TTL %q: %w", v, err)
		}
		cfg.CacheTTL = d
	}

	if v := os.Getenv("TARGET_CHAN_BUF"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("invalid TARGET_CHAN_BUF %q: must be positive integer", v)
		}
		cfg.TargetChanBuf = n
	}

	log.Printf("config: NFLOG_GROUP=%d IPSET_NAME=%s CACHE_TTL=%s TARGET_CHAN_BUF=%d",
		cfg.NFLogGroup, cfg.IPSetName, cfg.CacheTTL, cfg.TargetChanBuf)

	return cfg, nil
}
