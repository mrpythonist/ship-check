// Package checks defines the public contract every ShipCheck verification
// check must implement. Third-party checks import this package only; they
// never need to import internal packages.
package checks

import (
	"context"
	"encoding/json"
	"time"
)

// Status is the outcome of a single check.
type Status string

const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusWarn    Status = "warn"
	StatusError   Status = "error" // check itself errored (not a failed assertion)
	StatusSkipped Status = "skipped"
)

// Severity decides how a failed check affects the release verdict.
// A critical failure fails the release. A warning failure only warns.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
)

// Result is the structured outcome of one check execution.
type Result struct {
	CheckID        string         `json:"check_id"`
	Name           string         `json:"name"`
	Category       string         `json:"category"`
	Status         Status         `json:"status"`
	Severity       Severity       `json:"severity"`
	Duration       time.Duration  `json:"duration_ms"`
	Message        string         `json:"message"`
	Recommendation string         `json:"recommendation,omitempty"`
	Error          string         `json:"error,omitempty"`
	Details        map[string]any `json:"details,omitempty"`
}

// MarshalJSON renders Duration as whole milliseconds (the field is
// tagged duration_ms); the default time.Duration encoding is nanoseconds,
// which would mislabel the value.
func (r Result) MarshalJSON() ([]byte, error) {
	type resultJSON struct {
		CheckID        string         `json:"check_id"`
		Name           string         `json:"name"`
		Category       string         `json:"category"`
		Status         Status         `json:"status"`
		Severity       Severity       `json:"severity"`
		DurationMs     int64          `json:"duration_ms"`
		Message        string         `json:"message"`
		Recommendation string         `json:"recommendation,omitempty"`
		Error          string         `json:"error,omitempty"`
		Details        map[string]any `json:"details,omitempty"`
	}
	return json.Marshal(resultJSON{
		CheckID: r.CheckID, Name: r.Name, Category: r.Category,
		Status: r.Status, Severity: r.Severity,
		DurationMs: r.Duration.Milliseconds(), Message: r.Message,
		Recommendation: r.Recommendation, Error: r.Error, Details: r.Details,
	})
}

// Check is the contract for a verification check.
//
// Implementations must be safe to run against production: read-only,
// non-destructive, bounded by the context deadline, and must never log
// credentials. Run must not panic; the engine recovers panics and turns
// them into StatusError results.
type Check interface {
	ID() string
	Name() string
	Category() string
	Description() string
	Severity() Severity
	Timeout() time.Duration
	Run(ctx context.Context) Result
}

// Factory builds the checks for one category from parsed configuration.
// It is registered in the registry; the engine never imports concrete
// check packages directly.
type Factory func(cfg any) ([]Check, error)
