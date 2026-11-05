package registry

import (
	"testing"

	"github.com/mrpythonist/ship-check/pkg/checks"
)

func TestRegisterAndBuild(t *testing.T) {
	r := New()
	if err := r.Register("demo", Meta{Name: "Demo"}, func(cfg any) ([]checks.Check, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if err := r.Register("demo", Meta{Name: "Demo"}, func(cfg any) ([]checks.Check, error) { return nil, nil }); err == nil {
		t.Fatal("duplicate registration should error")
	}
	if _, err := r.Build("missing", nil); err == nil {
		t.Fatal("building an unregistered category should error")
	}
	if got := r.Categories(); len(got) != 1 || got[0] != "demo" {
		t.Fatalf("categories = %v", got)
	}
}
