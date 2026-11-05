package workers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

func TestHeartbeatFreshAndStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.txt")
	if err := os.WriteFile(path, []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.WorkersConfig{Enabled: true, HeartbeatFile: path, HeartbeatMaxAge: 5 * time.Minute, Timeout: 2 * time.Second}
	c, err := Factory(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res := c[0].Run(context.Background()); res.Status != checks.StatusPass {
		t.Fatalf("fresh heartbeat should pass, got %s: %s", res.Status, res.Message)
	}
	// Make it stale.
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if res := c[0].Run(context.Background()); res.Status != checks.StatusFail {
		t.Fatalf("stale heartbeat should fail, got %s: %s", res.Status, res.Message)
	}
}

func TestMissingProcessFails(t *testing.T) {
	cfg := config.WorkersConfig{Enabled: true, Timeout: 2 * time.Second,
		Processes: []config.WorkerProcess{{Name: "Nope", Pattern: "definitely-not-a-real-process-xyz123", MinCount: 1}}}
	c, err := Factory(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res := c[0].Run(context.Background()); res.Status != checks.StatusFail {
		t.Fatalf("missing process should fail, got %s", res.Status)
	}
}

func TestFactoryRequiresSomething(t *testing.T) {
	if _, err := Factory(config.WorkersConfig{Enabled: true}); err == nil {
		t.Fatal("workers with nothing configured should error")
	}
}
