// Package engine executes registered checks, enforces timeouts, and
// calculates the overall release verdict and exit code.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/internal/registry"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

// Exit codes (documented in README and docs/):
// 0 = release verified (pass, or warnings only)
// 1 = release FAILED (a critical check failed)
// 2 = configuration / usage error
// 3 = internal execution error (no checks could run)
const (
	ExitPass   = 0
	ExitFail   = 1
	ExitConfig = 2
	ExitError  = 3
)

// Verdict is the overall release status.
type Verdict string

const (
	VerdictPass    Verdict = "PASSED"
	VerdictFailed  Verdict = "FAILED"
	VerdictWarning Verdict = "WARNING"
	VerdictError   Verdict = "ERROR"
)

// Report is the structured result of one verification run.
type Report struct {
	Project     string          `json:"project"`
	Environment string          `json:"environment"`
	Timestamp   time.Time       `json:"timestamp"`
	Duration    time.Duration   `json:"duration_ms"`
	Verdict     Verdict         `json:"verdict"`
	ExitCode    int             `json:"exit_code"`
	Checks      []checks.Result `json:"checks"`
	Total       int             `json:"total"`
	Passed      int             `json:"passed"`
	Failed      int             `json:"failed"`
	Warnings    int             `json:"warnings"`
	Errors      int             `json:"errors"`
}

// MarshalJSON renders Duration as whole milliseconds, matching its
// duration_ms tag (time.Duration would otherwise encode nanoseconds).
func (rep Report) MarshalJSON() ([]byte, error) {
	type reportJSON struct {
		Project     string          `json:"project"`
		Environment string          `json:"environment"`
		Timestamp   time.Time       `json:"timestamp"`
		DurationMs  int64           `json:"duration_ms"`
		Verdict     Verdict         `json:"verdict"`
		ExitCode    int             `json:"exit_code"`
		Checks      []checks.Result `json:"checks"`
		Total       int             `json:"total"`
		Passed      int             `json:"passed"`
		Failed      int             `json:"failed"`
		Warnings    int             `json:"warnings"`
		Errors      int             `json:"errors"`
	}
	return json.Marshal(reportJSON{
		Project: rep.Project, Environment: rep.Environment, Timestamp: rep.Timestamp,
		DurationMs: rep.Duration.Milliseconds(), Verdict: rep.Verdict, ExitCode: rep.ExitCode,
		Checks: rep.Checks, Total: rep.Total, Passed: rep.Passed, Failed: rep.Failed,
		Warnings: rep.Warnings, Errors: rep.Errors,
	})
}

// Engine runs checks from a registry against a config.
type Engine struct {
	Reg        *registry.Registry
	FailOnWarn bool
}

// BuildChecks discovers checks for every enabled category.
func (e *Engine) BuildChecks(cfg *config.Config) ([]checks.Check, error) {
	var all []checks.Check
	for _, cat := range cfg.EnabledCategories() {
		var arg any
		switch cat {
		case "http":
			arg = cfg.Checks.HTTP
		case "database":
			arg = cfg.Checks.Database
		case "redis":
			arg = cfg.Checks.Redis
		case "docker":
			arg = cfg.Checks.Docker
		case "workers":
			arg = cfg.Checks.Workers
		case "system":
			arg = cfg.Checks.System
		}
		built, err := e.Reg.Build(cat, arg)
		if err != nil {
			return nil, fmt.Errorf("build %s checks: %w", cat, err)
		}
		all = append(all, built...)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no checks are enabled; enable at least one category in checks:")
	}
	return all, nil
}

// Run executes all checks with per-check timeouts and the global timeout.
func (e *Engine) Run(ctx context.Context, cfg *config.Config) (*Report, error) {
	list, err := e.BuildChecks(cfg)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, cfg.Verification.Timeout)
	defer cancel()

	results := make([]checks.Result, len(list))
	if cfg.Verification.Parallel {
		var wg sync.WaitGroup
		for i, c := range list {
			wg.Add(1)
			go func(i int, c checks.Check) {
				defer wg.Done()
				results[i] = runOne(ctx, c)
				if cfg.Verification.FailFast && results[i].Status == checks.StatusFail {
					cancel()
				}
			}(i, c)
		}
		wg.Wait()
	} else {
		for i, c := range list {
			results[i] = runOne(ctx, c)
			if cfg.Verification.FailFast && results[i].Status == checks.StatusFail {
				break
			}
		}
	}
	// Stable output order regardless of parallel completion.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Category == results[j].Category {
			return results[i].Name < results[j].Name
		}
		return results[i].Category < results[j].Category
	})
	rep := &Report{
		Project: cfg.Project.Name, Environment: cfg.Project.Environment,
		Timestamp: start, Duration: time.Since(start), Checks: results,
		Total: len(results),
	}
	for _, r := range results {
		switch r.Status {
		case checks.StatusPass:
			rep.Passed++
		case checks.StatusFail:
			rep.Failed++
		case checks.StatusWarn:
			rep.Warnings++
		case checks.StatusError:
			rep.Errors++
		}
	}
	rep.Verdict, rep.ExitCode = VerdictFor(rep, e.FailOnWarn)
	return rep, nil
}

// runOne executes a single check with its own timeout and panic recovery.
func runOne(parent context.Context, c checks.Check) (res checks.Result) {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			res = checks.Result{
				CheckID: c.ID(), Name: c.Name(), Category: c.Category(),
				Status: checks.StatusError, Severity: c.Severity(),
				Duration:       time.Since(start),
				Message:        "check panicked (bug in check implementation)",
				Error:          fmt.Sprintf("%v", r),
				Recommendation: "Report this as a bug; the check crashed instead of returning a result.",
			}
		}
	}()
	timeout := c.Timeout()
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	res = c.Run(ctx)
	res.Duration = time.Since(start)
	if res.CheckID == "" {
		res.CheckID = c.ID()
	}
	if res.Name == "" {
		res.Name = c.Name()
	}
	if res.Category == "" {
		res.Category = c.Category()
	}
	if res.Severity == "" {
		res.Severity = c.Severity()
	}
	// A context timeout the check did not translate itself becomes a failure.
	if ctx.Err() == context.DeadlineExceeded && res.Status == checks.StatusPass {
		res.Status = checks.StatusFail
		res.Message = fmt.Sprintf("timed out after %s", timeout)
		res.Recommendation = "Increase the check timeout, or investigate why the target is slow."
	}
	return res
}

// VerdictFor calculates the verdict and exit code from counted results.
// Critical failures fail the release. Warning-severity failures count as
// warnings, not failures. Errors (check crashed / could not run) fail the
// release, because an unverified release must not be trusted.
func VerdictFor(rep *Report, failOnWarn bool) (Verdict, int) {
	criticalFails := 0
	for _, r := range rep.Checks {
		if r.Status == checks.StatusFail && r.Severity == checks.SeverityCritical {
			criticalFails++
		}
		if r.Status == checks.StatusError {
			criticalFails++
		}
	}
	switch {
	case criticalFails > 0:
		return VerdictFailed, ExitFail
	case rep.Warnings > 0 && failOnWarn:
		return VerdictWarning, ExitFail
	case rep.Warnings > 0:
		return VerdictWarning, ExitPass
	case rep.Errors > 0:
		return VerdictError, ExitFail
	default:
		return VerdictPass, ExitPass
	}
}
