// Package reporting renders a verification Report as terminal text or JSON.
package reporting

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mrpythonist/ship-check/internal/engine"
	"github.com/mrpythonist/ship-check/pkg/checks"
)

// Render writes the report in format ("terminal" or "json") to w.
// JSON output is machine-readable and contains no terminal formatting.
func Render(w io.Writer, rep *engine.Report, format string, verbose bool) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	return renderTerminal(w, rep, verbose)
}

func renderTerminal(w io.Writer, rep *engine.Report, verbose bool) error {
	fmt.Fprintf(w, "ShipCheck v0.1.0\n\nProduction Release Verification\nProject: %s\nEnvironment: %s\n\n", rep.Project, rep.Environment)
	for _, r := range rep.Checks {
		label, width := statusLabel(r.Status), 0
		_ = width
		name := r.Name
		if len(name) > 34 {
			name = name[:34]
		}
		fmt.Fprintf(w, "[%s] %-34s %s\n", label, name, r.Duration.Round(0))
		if verbose || r.Status != checks.StatusPass {
			fmt.Fprintf(w, "       %s\n", r.Message)
			if r.Recommendation != "" {
				fmt.Fprintf(w, "       Recommendation: %s\n", r.Recommendation)
			}
		}
	}
	fmt.Fprintln(w, "\n-------------------------------------------")
	fmt.Fprintln(w, "Verification Summary")
	fmt.Fprintf(w, "\nTotal Checks: %d\nPassed:       %d\nFailed:       %d\nWarnings:     %d\nErrors:       %d\n", rep.Total, rep.Passed, rep.Failed, rep.Warnings, rep.Errors)
	fmt.Fprintf(w, "\nRelease Status: %s\n", rep.Verdict)
	// Surface the first actionable failure, like the spec example.
	for _, r := range rep.Checks {
		if r.Status == checks.StatusFail || r.Status == checks.StatusError {
			fmt.Fprintf(w, "\nIssue:\n%s\n", r.Message)
			if r.Recommendation != "" {
				fmt.Fprintf(w, "\nRecommendation:\n%s\n", r.Recommendation)
			}
			break
		}
	}
	fmt.Fprintln(w, "\n-------------------------------------------")
	fmt.Fprintf(w, "Exit Code: %d\n", rep.ExitCode)
	return nil
}

func statusLabel(s checks.Status) string {
	switch s {
	case checks.StatusPass:
		return "PASS"
	case checks.StatusFail:
		return "FAIL"
	case checks.StatusWarn:
		return "WARN"
	case checks.StatusError:
		return "ERROR"
	default:
		return strings.ToUpper(string(s))
	}
}
