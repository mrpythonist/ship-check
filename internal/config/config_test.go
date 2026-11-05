package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "shipcheck.yml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadValid(t *testing.T) {
	p := write(t, `
version: 1
project: {name: my-saas, environment: production}
verification: {timeout: 30s, parallel: true}
checks:
  http:
    enabled: true
    endpoints:
      - {name: API Health, url: https://example.com/api/health, expected_status: 200, timeout: 5s}
  redis: {enabled: true, address: 127.0.0.1:6379}
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Project.Name != "my-saas" || len(cfg.Checks.HTTP.Endpoints) != 1 {
		t.Fatalf("unexpected config: %+v", cfg.Project)
	}
	if got := cfg.EnabledCategories(); len(got) != 2 || got[0] != "http" || got[1] != "redis" {
		t.Fatalf("enabled categories = %v", got)
	}
}

func TestLoadInvalidVersionAndURL(t *testing.T) {
	p := write(t, `
version: 2
project: {name: x, environment: production}
checks:
  http: {enabled: true, endpoints: [{name: Bad, url: "not-a-url"}]}
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected validation error")
	} else {
		t.Logf("got expected error: %v", err)
	}
}

func TestLoadEnvExpansion(t *testing.T) {
	t.Setenv("SHIPCHECK_TEST_DSN", "postgres://u:p@localhost:5432/db?sslmode=disable")
	p := write(t, `
version: 1
project: {name: x, environment: production}
checks:
  database: {enabled: true, driver: postgres, dsn: "${SHIPCHECK_TEST_DSN}"}
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Checks.Database.DSN != "postgres://u:p@localhost:5432/db?sslmode=disable" {
		t.Fatalf("env not expanded: %q", cfg.Checks.Database.DSN)
	}
}

func TestValidateDestructiveQueryRejected(t *testing.T) {
	c := Default()
	c.Checks.Database.Enabled = true
	c.Checks.Database.Driver = "postgres"
	c.Checks.Database.Host = "localhost"
	c.Checks.Database.Query = "DROP TABLE users"
	if err := c.Validate(); err == nil {
		t.Fatal("expected destructive query to be rejected")
	}
}

func TestValidateBadDriver(t *testing.T) {
	c := Default()
	c.Checks.Database.Enabled = true
	c.Checks.Database.Driver = "sqlite"
	c.Checks.Database.Host = "localhost"
	if err := c.Validate(); err == nil {
		t.Fatal("expected bad driver to be rejected")
	}
}
