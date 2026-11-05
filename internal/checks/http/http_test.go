package httpcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

func run(t *testing.T, ep config.HTTPEndpoint) checks.Result {
	t.Helper()
	c, err := Factory(config.HTTPConfig{Enabled: true, Endpoints: []config.HTTPEndpoint{ep}})
	if err != nil {
		t.Fatal(err)
	}
	return c[0].Run(context.Background())
}

func TestPass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()
	res := run(t, config.HTTPEndpoint{Name: "API", URL: srv.URL, ExpectedStatus: 200, Timeout: 2 * time.Second, BodyContains: "ok"})
	if res.Status != checks.StatusPass {
		t.Fatalf("expected pass, got %s: %s", res.Status, res.Message)
	}
}

func TestWrongStatusFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	res := run(t, config.HTTPEndpoint{Name: "API", URL: srv.URL, ExpectedStatus: 200, Timeout: 2 * time.Second})
	if res.Status != checks.StatusFail {
		t.Fatalf("expected fail, got %s", res.Status)
	}
}

func TestBodyMismatchFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"error"}`))
	}))
	defer srv.Close()
	res := run(t, config.HTTPEndpoint{Name: "API", URL: srv.URL, ExpectedStatus: 200, Timeout: 2 * time.Second, BodyContains: "ok"})
	if res.Status != checks.StatusFail {
		t.Fatalf("expected fail on body mismatch, got %s", res.Status)
	}
}

func TestSlowEndpointTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	res := run(t, config.HTTPEndpoint{Name: "API", URL: srv.URL, ExpectedStatus: 200, Timeout: 100 * time.Millisecond})
	if res.Status != checks.StatusFail {
		t.Fatalf("expected fail on timeout, got %s: %s", res.Status, res.Message)
	}
}

func TestUnreachableFailsNotPanics(t *testing.T) {
	res := run(t, config.HTTPEndpoint{Name: "API", URL: "http://127.0.0.1:1/health", ExpectedStatus: 200, Timeout: time.Second})
	if res.Status != checks.StatusFail {
		t.Fatalf("expected fail, got %s", res.Status)
	}
	if res.Recommendation == "" {
		t.Fatal("failing check must carry a recommendation")
	}
}
