package service

import "testing"

func TestParsePIDList(t *testing.T) {
	pids, err := parsePIDList("1234 5678")
	if err != nil {
		t.Fatalf("parsePIDList: %v", err)
	}
	if len(pids) != 2 || pids[0] != 1234 || pids[1] != 5678 {
		t.Fatalf("got %v", pids)
	}
}

func TestParsePIDList_Empty(t *testing.T) {
	_, err := parsePIDList("   ")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}
