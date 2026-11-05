// Package systemcheck verifies host resources: disk usage, memory
// availability, CPU load, and required processes. Platform-specific
// reads are isolated here (Linux /proc + Statfs); on other platforms
// the checks return StatusError with a clear message instead of guessing.
package systemcheck

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

// Factory builds disk, memory, load and process checks as configured.
func Factory(cfg any) ([]checks.Check, error) {
	c, ok := cfg.(config.SystemConfig)
	if !ok {
		return nil, fmt.Errorf("system: unexpected config type %T", cfg)
	}
	var out []checks.Check
	out = append(out, &diskCheck{cfg: c}, &memoryCheck{cfg: c})
	if c.MaxLoad1m > 0 {
		out = append(out, &loadCheck{cfg: c})
	}
	for _, p := range c.Processes {
		p := p
		out = append(out, &procCheck{cfg: c, proc: p})
	}
	return out, nil
}

func severity(cfg config.SystemConfig) checks.Severity {
	// System resource checks warn by default: a full disk matters, but
	// it is usually not a failed deployment. Users can set severity: critical.
	if cfg.Severity == "critical" {
		return checks.SeverityCritical
	}
	return checks.SeverityWarning
}

func timeoutOf(cfg config.SystemConfig) time.Duration {
	if cfg.Timeout > 0 {
		return cfg.Timeout
	}
	return 5 * time.Second
}

type diskCheck struct{ cfg config.SystemConfig }

func (c *diskCheck) ID() string       { return "system:disk" }
func (c *diskCheck) Name() string     { return "Disk Usage" }
func (c *diskCheck) Category() string { return "system" }
func (c *diskCheck) Description() string {
	return "Verifies disk usage on the configured path is under the threshold."
}
func (c *diskCheck) Timeout() time.Duration    { return timeoutOf(c.cfg) }
func (c *diskCheck) Severity() checks.Severity { return severity(c.cfg) }

func (c *diskCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "system", Severity: c.Severity()}
	var st syscall.Statfs_t
	if err := syscall.Statfs(c.cfg.DiskPath, &st); err != nil {
		res.Status = checks.StatusError
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("could not read disk usage for %s: %v", c.cfg.DiskPath, err)
		return res
	}
	total := st.Blocks * uint64(st.Bsize)
	free := st.Bavail * uint64(st.Bsize)
	usedPct := 0.0
	if total > 0 {
		usedPct = float64(total-free) / float64(total) * 100
	}
	res.Details = map[string]any{"path": c.cfg.DiskPath, "used_percent": usedPct, "max_percent": c.cfg.DiskMaxUsagePct}
	if usedPct > c.cfg.DiskMaxUsagePct {
		res.Status = checks.StatusWarn
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("disk usage on %s is %.1f%% (max %.1f%%)", c.cfg.DiskPath, usedPct, c.cfg.DiskMaxUsagePct)
		res.Recommendation = "Free disk space or expand the volume before logs and uploads start failing."
		return res
	}
	res.Status = checks.StatusPass
	res.Duration = time.Since(start)
	res.Message = fmt.Sprintf("disk usage on %s is %.1f%%", c.cfg.DiskPath, usedPct)
	return res
}

type memoryCheck struct{ cfg config.SystemConfig }

func (c *memoryCheck) ID() string       { return "system:memory" }
func (c *memoryCheck) Name() string     { return "Memory Availability" }
func (c *memoryCheck) Category() string { return "system" }
func (c *memoryCheck) Description() string {
	return "Verifies available memory is above the threshold."
}
func (c *memoryCheck) Timeout() time.Duration    { return timeoutOf(c.cfg) }
func (c *memoryCheck) Severity() checks.Severity { return severity(c.cfg) }

func (c *memoryCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "system", Severity: c.Severity()}
	if runtime.GOOS != "linux" {
		res.Status = checks.StatusError
		res.Duration = time.Since(start)
		res.Message = "memory check is only implemented on Linux (/proc/meminfo)"
		return res
	}
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		res.Status = checks.StatusError
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("could not read /proc/meminfo: %v", err)
		return res
	}
	vals := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			v, _ := strconv.ParseUint(parts[1], 10, 64)
			vals[strings.TrimSuffix(parts[0], ":")] = v // KiB
		}
	}
	total, ok1 := vals["MemTotal"]
	avail, ok2 := vals["MemAvailable"]
	if !ok1 || !ok2 || total == 0 {
		res.Status = checks.StatusError
		res.Duration = time.Since(start)
		res.Message = "could not parse MemTotal/MemAvailable from /proc/meminfo"
		return res
	}
	usedPct := float64(total-avail) / float64(total) * 100
	res.Details = map[string]any{"used_percent": usedPct, "max_percent": c.cfg.MemoryMaxUsagePct}
	if usedPct > c.cfg.MemoryMaxUsagePct {
		res.Status = checks.StatusWarn
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("memory usage is %.1f%% (max %.1f%%)", usedPct, c.cfg.MemoryMaxUsagePct)
		res.Recommendation = "Check for memory leaks or oversized workers before the OOM killer intervenes."
		return res
	}
	res.Status = checks.StatusPass
	res.Duration = time.Since(start)
	res.Message = fmt.Sprintf("memory usage is %.1f%%", usedPct)
	return res
}

type loadCheck struct{ cfg config.SystemConfig }

func (c *loadCheck) ID() string       { return "system:load" }
func (c *loadCheck) Name() string     { return "CPU Load" }
func (c *loadCheck) Category() string { return "system" }
func (c *loadCheck) Description() string {
	return "Verifies 1-minute load average is under the threshold."
}
func (c *loadCheck) Timeout() time.Duration    { return timeoutOf(c.cfg) }
func (c *loadCheck) Severity() checks.Severity { return severity(c.cfg) }

func (c *loadCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "system", Severity: c.Severity()}
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		res.Status = checks.StatusError
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("could not read /proc/loadavg: %v", err)
		return res
	}
	parts := strings.Fields(string(data))
	load, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		res.Status = checks.StatusError
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("could not parse load average: %v", err)
		return res
	}
	res.Details = map[string]any{"load_1m": load, "max_load_1m": c.cfg.MaxLoad1m, "cpus": runtime.NumCPU()}
	if load > c.cfg.MaxLoad1m {
		res.Status = checks.StatusWarn
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("1-minute load is %.2f (max %.2f)", load, c.cfg.MaxLoad1m)
		res.Recommendation = "The host is heavily loaded; deployments may be slow or timing out."
		return res
	}
	res.Status = checks.StatusPass
	res.Duration = time.Since(start)
	res.Message = fmt.Sprintf("1-minute load is %.2f", load)
	return res
}

type procCheck struct {
	cfg  config.SystemConfig
	proc config.WorkerProcess
}

func (c *procCheck) ID() string       { return "system:process:" + c.proc.Name }
func (c *procCheck) Name() string     { return "Process: " + c.proc.Name }
func (c *procCheck) Category() string { return "system" }
func (c *procCheck) Description() string {
	return "Verifies a required process is running."
}
func (c *procCheck) Timeout() time.Duration    { return timeoutOf(c.cfg) }
func (c *procCheck) Severity() checks.Severity { return severity(c.cfg) }

func (c *procCheck) Run(ctx context.Context) checks.Result {
	start := time.Now()
	res := checks.Result{CheckID: c.ID(), Name: c.Name(), Category: "system", Severity: c.Severity()}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		res.Status = checks.StatusError
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("could not scan processes: %v", err)
		return res
	}
	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
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
			continue
		}
		if strings.Contains(string(cmdline), c.proc.Pattern) {
			found++
		}
	}
	min := c.proc.MinCount
	if min == 0 {
		min = 1
	}
	if found < min {
		res.Status = checks.StatusFail
		res.Duration = time.Since(start)
		res.Message = fmt.Sprintf("found %d process(es) matching %q, need at least %d", found, c.proc.Pattern, min)
		res.Recommendation = "Start the required process and check why it is not running."
		return res
	}
	res.Status = checks.StatusPass
	res.Duration = time.Since(start)
	res.Message = fmt.Sprintf("found %d process(es) matching %q", found, c.proc.Pattern)
	return res
}
