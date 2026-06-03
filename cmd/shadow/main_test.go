package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWaitForHTTP_ServerReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if !waitForHTTP(srv.URL, 2*time.Second) {
		t.Error("waitForHTTP should return true when the server returns 200")
	}
}

func TestWaitForHTTP_Timeout(t *testing.T) {
	// Port 1 is reserved and unbound; connections will be refused immediately.
	if waitForHTTP("http://127.0.0.1:1/health", 500*time.Millisecond) {
		t.Error("waitForHTTP should return false when the server is unreachable")
	}
}

func TestWaitForHTTP_Non200Response(t *testing.T) {
	// A server that always returns 503 should be treated as "not ready":
	// the HTTP listener exists, but the daemon's HTTP handler isn't
	// wired up yet (e.g. mid-startup).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if waitForHTTP(srv.URL, 500*time.Millisecond) {
		t.Error("waitForHTTP should return false when the server returns non-200")
	}
}

func TestWaitForHTTP_BecomesReadyMidPoll(t *testing.T) {
	// Server is "down" for the first 300ms, then "up". waitForHTTP
	// should keep polling and eventually succeed.
	var ready bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ready {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	go func() {
		time.Sleep(300 * time.Millisecond)
		ready = true
	}()

	if !waitForHTTP(srv.URL, 2*time.Second) {
		t.Error("waitForHTTP should succeed when the server becomes ready during the wait window")
	}
}
