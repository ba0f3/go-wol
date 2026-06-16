package ipset

import (
	"fmt"
	"log"
	"sync"

	"github.com/vishvananda/netlink"
)

// Resolver loads and caches IP-to-MAC mappings from an ipset hash:ip,mac set.
type Resolver struct {
	setName string
	mu      sync.RWMutex
	entries map[string]string
}

// NewResolver creates a resolver for the given ipset name.
func NewResolver(setName string) *Resolver {
	return &Resolver{
		setName: setName,
		entries: make(map[string]string),
	}
}

// Refresh loads entries from the kernel ipset via netlink and rebuilds the map.
func (r *Resolver) Refresh() error {
	log.Printf("ipset: refreshing set %q via netlink", r.setName)

	result, err := netlink.IpsetList(r.setName)
	if err != nil {
		return fmt.Errorf("ipset list %q: %w", r.setName, err)
	}

	entries := entriesFromResult(result)

	r.mu.Lock()
	r.entries = entries
	r.mu.Unlock()

	log.Printf("ipset: loaded %d entries from %q (type %s)", len(entries), r.setName, result.TypeName)
	return nil
}

// GetMac returns the MAC address for the given IP, if known.
func (r *Resolver) GetMac(ip string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mac, ok := r.entries[ip]
	return mac, ok
}

func entriesFromResult(result *netlink.IPSetResult) map[string]string {
	if result == nil {
		return nil
	}
	entries := make(map[string]string, len(result.Entries))
	for _, entry := range result.Entries {
		ip := entry.IP.To4()
		if ip == nil || len(entry.MAC) == 0 {
			continue
		}
		entries[ip.String()] = entry.MAC.String()
	}
	return entries
}
