package daemon

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// TestWaitForHTTPSuccess: the server is already up — we should return
// almost immediately.
func TestWaitForHTTPSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL)
	start := time.Now()
	if err := c.WaitForHTTP(2 * time.Second); err != nil {
		t.Fatalf("WaitForHTTP: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("WaitForHTTP took %v, want <500ms for an already-up server", elapsed)
	}
}

// TestWaitForHTTPRetriesUntilUp: the server returns 500 the first two
// times we probe, then 200. WaitForHTTP should keep retrying and
// eventually succeed because /api/dashboard is the same path it polls.
func TestWaitForHTTPRetriesUntilUp(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL)
	if err := c.WaitForHTTP(2 * time.Second); err != nil {
		t.Fatalf("WaitForHTTP: %v", err)
	}
	if hits < 3 {
		t.Errorf("hits = %d, want >=3 (should have retried past the 500s)", hits)
	}
}

// TestWaitForHTTPTimeout: the server never answers 2xx — we should time
// out within the budget we passed.
func TestWaitForHTTPTimeout(t *testing.T) {
	c := NewClientWithHTTP("http://127.0.0.1:1")
	start := time.Now()
	err := c.WaitForHTTP(300 * time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > 800*time.Millisecond {
		t.Errorf("WaitForHTTP took %v, want close to 300ms timeout", elapsed)
	}
}

// TestNewClientDefaults: NewClient must produce a client whose HTTPURL
// points at the documented default address, regardless of cwd.
func TestNewClientDefaults(t *testing.T) {
	c := NewClient()
	if c.HTTPURL() != DefaultHTTPAddress {
		t.Errorf("HTTPURL = %q, want %q", c.HTTPURL(), DefaultHTTPAddress)
	}
	// sockPath must end in shadow.sock (independent of cwd).
	if filepath.Base(c.sockPath) != "shadow.sock" {
		t.Errorf("sockPath = %q, want .../shadow.sock", c.sockPath)
	}
}
