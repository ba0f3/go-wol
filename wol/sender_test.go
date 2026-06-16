package wol

import (
	"net"
	"testing"
)

func TestBuildMagicPacket(t *testing.T) {
	mac, err := net.ParseMAC("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("ParseMAC: %v", err)
	}

	packet := BuildMagicPacket(mac)
	if len(packet) != 102 {
		t.Fatalf("packet length = %d, want 102", len(packet))
	}

	for i := 0; i < 6; i++ {
		if packet[i] != 0xff {
			t.Errorf("sync byte[%d] = %#x, want 0xff", i, packet[i])
		}
	}

	for i := 6; i < 102; i++ {
		want := mac[(i-6)%len(mac)]
		if packet[i] != want {
			t.Errorf("byte[%d] = %#x, want %#x", i, packet[i], want)
			break
		}
	}
}
