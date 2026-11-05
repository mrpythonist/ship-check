package engine

import (
	"context"
	"testing"
	"time"

	"github.com/mrpythonist/ship-check/internal/config"
	"github.com/mrpythonist/ship-check/internal/registry"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

type fakeCheck struct {
	id       string
	name     string
	category string
	sev      checks.Severity
	status   checks.Status
	delay    time.Duration
}

func (f *fakeCheck) ID() string          { return f.id }
func (f *fakeCheck) Name() string        { return f.name }
func (f *fakeCheck) Category() string    { return f.category }
func (f *fakeCheck) Description() string { return "fake" }
func (f *fakeCheck) Severity() checks.Severity {
	if f.sev != "" {
		return f.sev
	}
	return checks.SeverityCritical
}
func (f *fakeCheck) Timeout() time.Duration { return 2 * time.Second }
func (f *fakeCheck) Run(ctx context.Context) checks.Result {
	select {
	case <-ctx.Done():
		return checks.Result{CheckID: f.id, Name: f.name, Category: f.category, Status: checks.StatusFail, Message: "timed out"}
	case <-time.After(f.delay):
		return checks.Result{CheckID: f.id, Name: f.name, Category: f.category, Status: f.status, Message: string(f.status), Severity: f.Severity()}
	}
}

func testEngine(t *testing.T, list []checks.Check) *Engine {
	t.Helper()
	r := registry.New()
	_ = r.Register("http", registry.Meta{Name: "HTTP"}, func(cfg any) ([]checks.Check, error) { return list, nil })
	return &Engine{Reg: r}
}

func baseCfg() *config.Config {
	c := config.Default()
	c.Checks.HTTP.Enabled = true
	return c
}

func TestRunPassAndFail(t *testing.T) {
	e := testEngine(t, []checks.Check{
		&fakeCheck{id: "a", name: "A", category: "http", status: checks.StatusPass},
		&fakeCheck{id: "b", name: "B", category: "http", status: checks.StatusFail},
	})
	rep, err := e.Run(context.Background(), baseCfg())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Verdict != VerdictFailed || rep.ExitCode != ExitFail || rep.Passed != 1 || rep.Failed != 1 {
		t.Fatalf("verdict=%s exit=%d passed=%d failed=%d", rep.Verdict, rep.ExitCode, rep.Passed, rep.Failed)
	}
}

func TestWarningDoesNotFailUnlessStrict(t *testing.T) {
	e := testEngine(t, []checks.Check{
		&fakeCheck{id: "a", name: "A", category: "http", status: checks.StatusWarn, sev: checks.SeverityWarning},
	})
	rep, _ := e.Run(context.Background(), baseCfg())
	if rep.Verdict != VerdictWarning || rep.ExitCode != ExitPass {
		t.Fatalf("warning should not fail: verdict=%s exit=%d", rep.Verdict, rep.ExitCode)
	}
	e.FailOnWarn = true
	rep, _ = e.Run(context.Background(), baseCfg())
	if rep.ExitCode != ExitFail {
		t.Fatalf("fail-on-warn should exit 1, got %d", rep.ExitCode)
	}
}

func TestParallelAndTimeout(t *testing.T) {
	e := testEngine(t, []checks.Check{
		&fakeCheck{id: "slow", name: "Slow", category: "http", status: checks.StatusPass, delay: 5 * time.Second},
		&fakeCheck{id: "fast", name: "Fast", category: "http", status: checks.StatusPass},
	})
	start := time.Now()
	rep, _ := e.Run(context.Background(), baseCfg())
	if time.Since(start) > 4*time.Second {
		t.Fatalf("parallel run took too long: %v", time.Since(start))
	}
	if rep.Failed != 1 {
		t.Fatalf("slow check should have timed out and failed, failed=%d", rep.Failed)
	}
}

func TestNoChecksEnabled(t *testing.T) {
	e := testEngine(t, nil)
	c := config.Default()
	c.Checks.HTTP.Enabled = false
	if _, err := e.Run(context.Background(), c); err == nil {
		t.Fatal("expected error when no checks enabled")
	}
}

func TestPanicBecomesError(t *testing.T) {
	panicking := &panicCheck{}
	e := testEngine(t, []checks.Check{panicking})
	rep, err := e.Run(context.Background(), baseCfg())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Errors != 1 || rep.ExitCode != ExitFail {
		t.Fatalf("panic should become an error result, errors=%d exit=%d", rep.Errors, rep.ExitCode)
	}
}

type panicCheck struct{}

func (p *panicCheck) ID() string                { return "panic" }
func (p *panicCheck) Name() string              { return "Panic" }
func (p *panicCheck) Category() string          { return "http" }
func (p *panicCheck) Description() string       { return "panics" }
func (p *panicCheck) Severity() checks.Severity { return checks.SeverityCritical }
func (p *panicCheck) Timeout() time.Duration    { return time.Second }
func (p *panicCheck) Run(ctx context.Context) checks.Result {
	panic("boom")
}
