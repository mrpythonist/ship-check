package systemcheck

import (
	"context"
	"testing"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

func TestDiskCheckPassesOnRoot(t *testing.T) {
	c, err := Factory(config.SystemConfig{Enabled: true, DiskPath: "/", DiskMaxUsagePct: 100, MemoryMaxUsagePct: 100, Timeout: 2000000000})
	if err != nil {
		t.Fatal(err)
	}
	// c[0] is disk, c[1] is memory.
	if res := c[0].Run(context.Background()); res.Status != checks.StatusPass {
		t.Fatalf("disk / with 100%% max should pass, got %s: %s", res.Status, res.Message)
	}
	if res := c[1].Run(context.Background()); res.Status == checks.StatusError {
		t.Fatalf("memory check errored: %s", res.Message)
	}
}

func TestDiskCheckWarnsAtZeroThreshold(t *testing.T) {
	c, _ := Factory(config.SystemConfig{Enabled: true, DiskPath: "/", DiskMaxUsagePct: 0, MemoryMaxUsagePct: 100})
	// DiskMaxUsagePct 0 means any usage warns (used as a threshold test).
	res := c[0].Run(context.Background())
	if res.Status != checks.StatusWarn {
		t.Fatalf("disk with 0%% threshold should warn, got %s", res.Status)
	}
}
