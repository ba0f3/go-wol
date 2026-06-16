package ipset

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func TestEntriesFromResult(t *testing.T) {
	result := &netlink.IPSetResult{
		TypeName: "hash:ip,mac",
		Entries: []netlink.IPSetEntry{
			{
				IP:  net.IPv4(192, 168, 10, 5),
				MAC: mustMAC(t, "AA:BB:CC:DD:EE:FF"),
			},
			{
				IP:  net.IPv4(192, 168, 10, 6),
				MAC: mustMAC(t, "11:22:33:44:55:66"),
			},
			{
				IP:  net.ParseIP("2001:db8::1"),
				MAC: mustMAC(t, "AA:BB:CC:DD:EE:FF"),
			},
			{
				IP:  net.IPv4(10, 0, 0, 1),
				MAC: nil,
			},
		},
	}

	entries := entriesFromResult(result)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries["192.168.10.5"] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("192.168.10.5 MAC = %q", entries["192.168.10.5"])
	}
	if entries["192.168.10.6"] != "11:22:33:44:55:66" {
		t.Errorf("192.168.10.6 MAC = %q", entries["192.168.10.6"])
	}
}

func TestResolver_GetMac(t *testing.T) {
	r := NewResolver("test")
	r.entries = map[string]string{"10.0.0.1": "AA:BB:CC:DD:EE:FF"}

	mac, ok := r.GetMac("10.0.0.1")
	if !ok || mac != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("GetMac = (%q, %v), want (AA:BB:CC:DD:EE:FF, true)", mac, ok)
	}

	_, ok = r.GetMac("10.0.0.2")
	if ok {
		t.Error("expected false for unknown IP")
	}
}

func mustMAC(t *testing.T, s string) net.HardwareAddr {
	t.Helper()
	mac, err := net.ParseMAC(s)
	if err != nil {
		t.Fatalf("ParseMAC(%q): %v", s, err)
	}
	return mac
}
