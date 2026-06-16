package privileges

import (
	"os"
	"testing"
)

func TestCanUseNetfilter_AsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
	if !CanUseNetfilter() {
		t.Fatal("expected root process to have netfilter privileges")
	}
}

func TestEffectiveCaps(t *testing.T) {
	_, err := effectiveCaps()
	if err != nil {
		t.Fatalf("effectiveCaps: %v", err)
	}
}
