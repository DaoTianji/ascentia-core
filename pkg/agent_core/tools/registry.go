package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Registry is a name-indexed sandbox of tools for one request.
type Registry struct {
	byName map[string]Tool
}

// NewRegistry builds a registry; duplicate names return an error.
func NewRegistry(ts []Tool) (*Registry, error) {
	byName := make(map[string]Tool, len(ts))
	for _, t := range ts {
		n := t.Name()
		if n == "" {
			return nil, fmt.Errorf("tools: empty tool name")
		}
		if _, dup := byName[n]; dup {
			return nil, fmt.Errorf("tools: duplicate tool %q", n)
		}
		byName[n] = t
	}
	return &Registry{byName: byName}, nil
}

// Definitions for the model API.
func (r *Registry) Definitions() []Definition {
	out := make([]Definition, 0, len(r.byName))
	for _, t := range r.byName {
		out = append(out, t.Definition())
	}
	return out
}

// Execute runs a tool if registered and allowlisted.
func (r *Registry) Execute(ctx context.Context, tcx Context, name string, input json.RawMessage) (string, error) {
	if err := ensureAllowed(tcx, name); err != nil {
		return "", err
	}
	t, ok := r.byName[name]
	if !ok {
		return "", fmt.Errorf("tools: unknown tool %q", name)
	}
	return t.Execute(ctx, tcx, input)
}

// Has reports whether the tool is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.byName[name]
	return ok
}
