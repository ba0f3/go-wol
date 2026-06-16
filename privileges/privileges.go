package privileges

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	capNetAdmin = 12
	capNetRaw   = 13
)

// CanUseNetfilter reports whether the process can bind NFLOG and use netlink.
func CanUseNetfilter() bool {
	if os.Geteuid() == 0 {
		return true
	}

	eff, err := effectiveCaps()
	if err != nil {
		return false
	}

	return eff[capNetAdmin] && eff[capNetRaw]
}

// NetfilterError returns a actionable error when netfilter privileges are missing.
func NetfilterError() error {
	return fmt.Errorf(`insufficient privileges for NFLOG (requires root or file capabilities)

  run as root:  sudo ./go-wol
  or set caps:  sudo setcap cap_net_admin,cap_net_raw=ep ./go-wol

  systemd: ensure the service runs as root (default for "go-wol service install")`)
}

func effectiveCaps() (map[int]bool, error) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return nil, err
	}

	caps := make(map[int]bool)
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "CapEff:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("parse CapEff line")
		}

		mask, err := strconv.ParseUint(fields[1], 16, 64)
		if err != nil {
			return nil, fmt.Errorf("parse CapEff value: %w", err)
		}

		for i := 0; i < 64; i++ {
			if mask&(1<<uint(i)) != 0 {
				caps[i] = true
			}
		}
		return caps, nil
	}

	return nil, fmt.Errorf("CapEff not found in /proc/self/status")
}
