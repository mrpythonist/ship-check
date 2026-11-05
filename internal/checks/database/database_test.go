package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

func TestUnreachablePostgresFailsWithoutLeakingPassword(t *testing.T) {
	c, err := Factory(config.DatabaseConfig{
		Enabled: true, Driver: "postgres", Host: "127.0.0.1", Port: 1,
		User: "u", Password: "s3cret-password", Database: "db", Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	res := c[0].Run(context.Background())
	if res.Status != checks.StatusFail {
		t.Fatalf("expected fail, got %s", res.Status)
	}
	if strings.Contains(res.Message, "s3cret-password") {
		t.Fatalf("password leaked in result: %q", res.Message)
	}
}

func TestUnreachableMySQLFails(t *testing.T) {
	c, _ := Factory(config.DatabaseConfig{
		Enabled: true, Driver: "mysql", Host: "127.0.0.1", Port: 1,
		User: "u", Password: "x", Database: "db", Timeout: time.Second,
	})
	res := c[0].Run(context.Background())
	if res.Status != checks.StatusFail {
		t.Fatalf("expected fail, got %s: %s", res.Status, res.Message)
	}
}
