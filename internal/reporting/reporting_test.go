package reporting

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mrpythonist/ship-check/internal/engine"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

func sampleReport() *engine.Report {
	return &engine.Report{
		Project: "my-saas", Environment: "production", Timestamp: time.Now(),
		Verdict: engine.VerdictFailed, ExitCode: 1, Total: 2, Passed: 1, Failed: 1,
		Checks: []checks.Result{
			{CheckID: "http:a", Name: "API Health", Category: "http", Status: checks.StatusPass, Message: "HTTP 200"},
			{CheckID: "workers:b", Name: "Worker", Category: "workers", Status: checks.StatusFail, Message: "heartbeat stale", Recommendation: "Restart the worker."},
		},
	}
}

func TestJSONIsValid(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, sampleReport(), "json", false); err != nil {
		t.Fatal(err)
	}
	var decoded engine.Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON output is not valid: %v\n%s", err, buf.String())
	}
	if decoded.Verdict != engine.VerdictFailed || decoded.Total != 2 {
		t.Fatalf("decoded report wrong: %+v", decoded)
	}
}

func TestTerminalShowsFailureAndRecommendation(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, sampleReport(), "terminal", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"[FAIL]", "Release Status: FAILED", "Exit Code: 1", "Restart the worker."} {
		if !strings.Contains(out, want) {
			t.Fatalf("terminal output missing %q:\n%s", want, out)
		}
	}
}
