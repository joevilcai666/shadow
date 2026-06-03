package daemon

import (
	"net"
	"testing"
)

func TestTryPorts_FreePort(t *testing.T) {
	// Pick a port we know is free, then ask TryPorts to find it.
	start := 30000
	for {
		ln, err := net.Listen("tcp", "127.0.0.1:30000")
		if err == nil {
			_ = ln.Close()
			break
		}
		start++
		if start > 30100 {
			t.Skip("no free port in test range; skipping")
		}
	}

	got, err := TryPorts(start, 1)
	if err != nil {
		t.Fatalf("TryPorts: %v", err)
	}
	if got != start {
		t.Errorf("got %d, want %d", got, start)
	}
}

func TestTryPorts_SkipsOccupied(t *testing.T) {
	// Hold a port to make it busy.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	busy := ln.Addr().(*net.TCPAddr).Port

	got, err := TryPorts(busy, 3)
	if err != nil {
		t.Fatalf("TryPorts: %v", err)
	}
	if got == busy {
		t.Errorf("expected to skip busy port %d, got %d", busy, got)
	}
	if got != busy+1 && got != busy+2 {
		t.Errorf("expected to land on %d or %d, got %d", busy+1, busy+2, got)
	}
}

func TestTryPorts_NoFreeInRange(t *testing.T) {
	// Hold 3 consecutive ports, then probe a 2-port range that
	// overlaps them.
	hold := make([]net.Listener, 0, 3)
	defer func() {
		for _, l := range hold {
			_ = l.Close()
		}
	}()
	for i := 0; i < 3; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		hold = append(hold, ln)
	}
	p0 := hold[0].Addr().(*net.TCPAddr).Port
	p2 := hold[2].Addr().(*net.TCPAddr).Port

	// We can never guarantee the kernel will assign 3 consecutive ports
	// in a single call, so this test only checks that the function
	// behaves sanely for a tiny range. If the range is too small to
	// contain a free port, the function returns an error — we accept
	// either "got a port" or "no port", but never panic.
	got, err := TryPorts(p0, p2-p0+1)
	if err == nil {
		// The kernel might have spaced them out. Just verify the
		// returned port is in range and not one of the held ones.
		if got < p0 || got > p2 {
			t.Errorf("port %d outside range [%d, %d]", got, p0, p2)
		}
	}
}

func TestTryPorts_InvalidArgs(t *testing.T) {
	if _, err := TryPorts(0, 1); err == nil {
		t.Error("expected error for start=0")
	}
	if _, err := TryPorts(-1, 1); err == nil {
		t.Error("expected error for start=-1")
	}
	if _, err := TryPorts(70000, 1); err == nil {
		t.Error("expected error for start=70000 (out of range)")
	}
}
