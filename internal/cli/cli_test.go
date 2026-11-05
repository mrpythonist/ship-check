package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrpythonist/ship-check/internal/engine"
)

func TestInitAndVerifyAgainstRealServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "shipcheck.yml")
	var out, errBuf bytes.Buffer
	if code := Run([]string{"init", "--config", cfgPath}, &out, &errBuf); code != engine.ExitPass {
		t.Fatalf("init exit %d: %s", code, errBuf.String())
	}
	// Replace the example endpoint with the test server.
	data, _ := os.ReadFile(cfgPath)
	data = []byte(strings.Replace(string(data),
		"url: https://example.com/api/health", "url: "+srv.URL, 1))
	data = []byte(strings.Replace(string(data), "body_contains: \"ok\"", "body_contains: \"ok\"", 1))
	// Disable TLS check for the plain-http test server.
	data = []byte(strings.Replace(string(data), "check_tls: true", "check_tls: false", 1))
	// Disable the other categories explicitly (they are false in the example already).
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	// Run from the temp dir so findConfig is not needed; pass --config.
	oldWD, _ := os.Getwd()
	defer os.Chdir(oldWD)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	code := Run([]string{"verify", "--config", cfgPath}, &out, &errBuf)
	if code != engine.ExitPass {
		t.Fatalf("verify exit %d, output:\n%s\nstderr:\n%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), "[PASS]") || !strings.Contains(out.String(), "Release Status: PASSED") {
		t.Fatalf("verify output missing PASS:\n%s", out.String())
	}
}

func TestVerifyBadConfigExits2(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "shipcheck.yml")
	os.WriteFile(p, []byte("version: 99\nproject: {name: x, environment: production}\n"), 0644)
	var out, errBuf bytes.Buffer
	if code := Run([]string{"verify", "--config", p}, &out, &errBuf); code != engine.ExitConfig {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestListChecksAndVersion(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := Run([]string{"list-checks"}, &out, &errBuf); code != engine.ExitPass {
		t.Fatalf("list-checks exit %d", code)
	}
	if !strings.Contains(out.String(), "http") || !strings.Contains(out.String(), "workers") {
		t.Fatalf("list-checks missing categories:\n%s", out.String())
	}
	out.Reset()
	if code := Run([]string{"version"}, &out, &errBuf); code != engine.ExitPass || !strings.Contains(out.String(), "shipcheck") {
		t.Fatalf("version failed: %d %q", code, out.String())
	}
}
