// Package registry is the check plugin registry. Categories register a
// factory; the engine discovers checks only through this registry, so a
// third-party check package can plug in without touching the core.
package registry

import (
	"fmt"
	"sort"

	"github.com/mrpythonist/ship-check/pkg/checks"
)

// Registry maps a category name (http, database, ...) to its factory.
type Registry struct {
	factories map[string]checks.Factory
	meta      map[string]Meta
}

// Meta describes a category for list-checks / doctor output.
type Meta struct {
	Category    string
	Name        string
	Description string
}

func New() *Registry {
	return &Registry{factories: map[string]checks.Factory{}, meta: map[string]Meta{}}
}

// Register adds a factory. Registering the same category twice is an error.
func (r *Registry) Register(category string, m Meta, f checks.Factory) error {
	if _, ok := r.factories[category]; ok {
		return fmt.Errorf("registry: category %q already registered", category)
	}
	m.Category = category
	r.factories[category] = f
	r.meta[category] = m
	return nil
}

// Build runs the factory for category with cfg, returning its checks.
func (r *Registry) Build(category string, cfg any) ([]checks.Check, error) {
	f, ok := r.factories[category]
	if !ok {
		return nil, fmt.Errorf("registry: no checks registered for category %q", category)
	}
	return f(cfg)
}

// Categories returns registered categories in sorted order.
func (r *Registry) Categories() []string {
	out := make([]string, 0, len(r.factories))
	for k := range r.factories {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Describe returns metadata for a category, or false if unregistered.
func (r *Registry) Describe(category string) (Meta, bool) {
	m, ok := r.meta[category]
	return m, ok
}
