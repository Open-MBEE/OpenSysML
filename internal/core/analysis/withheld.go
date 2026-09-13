package analysis

import (
	"context"
	"errors"
	"fmt"
)

// ServeAll is the spelling that serves every manifest engine.
const ServeAll = "all"

// ErrEngineWithheld is the typed error for a manifest engine a service lists but does not
// run: a request naming it asks to run a program on the server the operator did not grant.
var ErrEngineWithheld = errors.New("engine is not served by this service")

// EngineWithheldError names the engine a service withholds.
type EngineWithheldError struct {
	Name string
}

// Error reads `engine 'spin-bridge' is not served by this service`.
func (e *EngineWithheldError) Error() string {
	return fmt.Sprintf("engine '%s' is not served by this service", e.Name)
}

// Is matches ErrEngineWithheld.
func (e *EngineWithheldError) Is(target error) bool { return target == ErrEngineWithheld }

// ErrNotExternal is the typed error for a name given to serve that is not a manifest engine.
var ErrNotExternal = errors.New("not an external engine")

// NotExternalError reports a name asked to be served that names no manifest engine: a
// built-in, a tool or nothing at all.
type NotExternalError struct {
	Name string
	// Known lists the manifest engines that can be served.
	Known []string
}

// Error names the engine and the ones that could have been meant.
func (e *NotExternalError) Error() string {
	return fmt.Sprintf("%q is not an external engine; the manifests register %v", e.Name, e.Known)
}

// Is matches ErrNotExternal.
func (e *NotExternalError) Is(target error) bool { return target == ErrNotExternal }

// Withheld is an engine a registry lists but does not run, with the typed reason.
type Withheld interface {
	Engine
	Withheld() error
}

// withheldEngine lists as the manifest engine it wraps and refuses every question.
type withheldEngine struct {
	Manifested
}

// Withheld is the refusal every question meets.
func (w withheldEngine) Withheld() error { return &EngineWithheldError{Name: w.Name()} }

// Covers refuses: the engine is listed, not served.
func (w withheldEngine) Covers(*Model, Question) Coverage {
	return Coverage{Refusal: w.Withheld()}
}

// Run is never reached through Covers; asked directly it refuses the same way.
func (w withheldEngine) Run(context.Context, *Model, Question, Budget) (Result, error) {
	return Result{}, w.Withheld()
}

// External is the engine's manifest entries: every Manifested engine that is not a tool.
func (r *Registry) External() []Manifested {
	var external []Manifested
	for _, e := range r.Engines() {
		if m, ok := e.(Manifested); ok && m.Origin().Kind != KindTool {
			external = append(external, m)
		}
	}
	return external
}

// Serving returns the registry as a service runs it: every manifest engine not named is
// withheld — listed, refused when selected or reached — and `all` names every one. A
// name that is not a manifest engine is a NotExternalError; tools and built-ins are
// served as they are.
func (r *Registry) Serving(names []string) (*Registry, error) {
	external := r.External()
	known := make([]string, len(external))
	for i, m := range external {
		known[i] = m.Name()
	}
	serve := make(map[string]bool, len(names))
	for _, name := range names {
		if name == ServeAll {
			for _, k := range known {
				serve[k] = true
			}
			continue
		}
		found := false
		for _, k := range known {
			found = found || k == name
		}
		if !found {
			return nil, &NotExternalError{Name: name, Known: known}
		}
		serve[name] = true
	}
	served := NewRegistry()
	for _, e := range r.Engines() {
		m, ok := e.(Manifested)
		if ok && m.Origin().Kind != KindTool && !serve[e.Name()] {
			e = withheldEngine{Manifested: m}
		}
		served.engines[e.Name()] = e
	}
	return served, nil
}
