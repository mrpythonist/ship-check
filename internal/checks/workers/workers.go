// Package workers verifies background workers three ways, because a
// running process alone does not prove jobs are being processed:
//  1. process availability (a matching process exists, /proc scan, no shell)
//  2. heartbeat freshness (a file the worker touches regularly)
//  3. queue backlog (Redis list length under a threshold, where configured)
//
// Laravel Horizon note: Horizon stores its state in Redis; point a queue
// check at the Horizon queue key (e.g. queues:default) and a heartbeat
// check at Horizon's supervisor heartbeat if you run one. See docs/CHECKS.md.
package workers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mrpythonist/ship-check/internal/checks/redis"
	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

// Factory builds one check per process, plus heartbeat and queue checks.
func Factory(cfg any) ([]checks.Check, error) {
	c, ok := cfg.(config.WorkersConfig)
	if !ok {
		return nil, fmt.Errorf("workers: unexpected config type %T", cfg)
	}
	var out []checks.Check
	for _, p := range c.Processes {
		p := p
		out = append(out, &processCheck{cfg: c, proc: p})
	}
	if c.HeartbeatFile != "" {
		out = append(out, &heartbeatCheck{cfg: c})
	}
	for _, q := range c.Queues {
		q := q
		out = append(out, &queueCheck{cfg: c, queue: q})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("workers: enabled but no processes, heartbeat_file, or queues are configured")
	}
	return out, nil
}

func severity(cfg config.WorkersConfig) checks.Severity {
	if cfg.Severity == "warning" {
		return checks.SeverityWarning
	}
	return checks.SeverityCritical
}

func timeoutOf(cfg config.WorkersConfig) time.Duration {
	if cfg.Timeout > 0 {
		return cfg.Timeout
	}
	return 5 * time.Second
}

// countProcesses scans /proc/*/cmdline for a substring match. No shell is
// involved, and the scan is read-only.
func countProcesses(pattern string) (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// PID directories are numeric.
		isPID := true
		for _, r := range e.Name() {
			if r < '0' || r > '9' {
				isPID = false
				break
			}
		}
		if !isPID {
			continue
		}
		cmdline, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err != nil {
			continue // process exited mid-scan; ignore
		}
		if strings.Contains(string(cmdline), pattern) {
			n++
		}
	}
	return n, nil
}

type processCheck struct {
	cfg  config.WorkersConfig
	proc config.WorkerProcess
}

func (c *processCheck) ID() string       { return "workers:process:" + c.proc.Name }
func (c *processCheck) Name() string     { return "Worker: " + c.proc.Name }
func (c *processCheck) Category() string { return "workers" }
func (c *processCheck) Description() string {
	return "Verifies at least one worker process matching the pattern is running."
}
func (c *processCheck) Timeout() time.Duration    { return timeoutOf(c.cfg) }
func (c *processCheck) Severity() checks.Severity { return severity(c.cfg) }

func (c *processCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "workers", Severity: c.Severity()}
	min := c.proc.MinCount
	if min == 0 {
		min = 1
	}
	n, err := countProcesses(c.proc.Pattern)
	if err != nil {
		res.Status = checks.StatusError
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("could not scan processes: %v", err)
		res.Recommendation = "ShipCheck scans /proc; run it on Linux or configure a heartbeat check instead."
		return res
	}
	res.Details = map[string]any{"pattern": c.proc.Pattern, "found": n, "min_count": min}
	if n < min {
		res.Status = checks.StatusFail
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("found %d process(es) matching %q, need at least %d", n, c.proc.Pattern, min)
		res.Recommendation = "Restart the queue worker (e.g. `php artisan queue:work` or your supervisor) and check its logs."
		return res
	}
	res.Status = checks.StatusPass
	res.Duration = time.Since(start)
	res.Message = fmt.Sprintf("found %d worker process(es) matching %q", n, c.proc.Pattern)
	return res
}

type heartbeatCheck struct{ cfg config.WorkersConfig }

func (c *heartbeatCheck) ID() string       { return "workers:heartbeat" }
func (c *heartbeatCheck) Name() string     { return "Worker Heartbeat" }
func (c *heartbeatCheck) Category() string { return "workers" }
func (c *heartbeatCheck) Description() string {
	return "Verifies the worker heartbeat file was updated recently, proving the worker is actually processing."
}
func (c *heartbeatCheck) Timeout() time.Duration    { return timeoutOf(c.cfg) }
func (c *heartbeatCheck) Severity() checks.Severity { return severity(c.cfg) }

func (c *heartbeatCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "workers", Severity: c.Severity()}
	maxAge := c.cfg.HeartbeatMaxAge
	if maxAge == 0 {
		maxAge = 5 * time.Minute
	}
	info, err := os.Stat(c.cfg.HeartbeatFile)
	if err != nil {
		res.Status = checks.StatusFail
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("heartbeat file %s is missing: %v", c.cfg.HeartbeatFile, err)
		res.Recommendation = "The worker has never written its heartbeat, or the path is wrong; check the worker writes the file on every job or every minute."
		return res
	}
	age := time.Since(info.ModTime())
	res.Details = map[string]any{"heartbeat_file": c.cfg.HeartbeatFile, "age_seconds": int(age.Seconds()), "max_age_seconds": int(maxAge.Seconds())}
	if age > maxAge {
		res.Status = checks.StatusFail
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("worker heartbeat is %s old (max %s); the worker may be stuck", age.Round(time.Second), maxAge)
		res.Recommendation = "Check the queue worker process and restart it if necessary; a running process with a stale heartbeat is not processing jobs."
		return res
	}
	res.Status = checks.StatusPass
	res.Duration = time.Since(start)
	res.Message = fmt.Sprintf("worker heartbeat is fresh (%s old)", age.Round(time.Second))
	return res
}

type queueCheck struct {
	cfg   config.WorkersConfig
	queue config.WorkerQueue
}

func (c *queueCheck) ID() string       { return "workers:queue:" + c.queue.Name }
func (c *queueCheck) Name() string     { return "Queue: " + c.queue.Name }
func (c *queueCheck) Category() string { return "workers" }
func (c *queueCheck) Description() string {
	return "Verifies a Redis queue backlog is under the configured threshold."
}
func (c *queueCheck) Timeout() time.Duration    { return timeoutOf(c.cfg) }
func (c *queueCheck) Severity() checks.Severity { return severity(c.cfg) }

func (c *queueCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "workers", Severity: c.Severity()}
	addr := c.queue.Address
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	n, err := redischeck.LLEN(ctx, addr, "", 0, c.queue.Key, c.Timeout())
	if err != nil {
		res.Status = checks.StatusFail
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("could not read queue %q length from Redis %s: %v", c.queue.Key, addr, err)
		res.Recommendation = "Check Redis is reachable and the queue key is correct."
		return res
	}
	res.Details = map[string]any{"queue": c.queue.Key, "length": n, "max_length": c.queue.MaxLength}
	if n > c.queue.MaxLength {
		res.Status = checks.StatusFail
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("queue %q has %d pending jobs (max %d)", c.queue.Key, n, c.queue.MaxLength)
		res.Recommendation = "Jobs are accumulating; scale workers or investigate why processing is slow."
		return res
	}
	res.Status = checks.StatusPass
	res.Duration = time.Since(start)
	res.Message = fmt.Sprintf("queue %q backlog is %d job(s)", c.queue.Key, n)
	return res
}
