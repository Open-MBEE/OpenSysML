package analysis

import (
	"sort"
)

// Registry holds the engines one owner answers with. There is no package-level registry:
// a binary builds one with Default, a test with NewRegistry, and neither sees the other's.
type Registry struct {
	engines map[string]Engine
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{engines: make(map[string]Engine)}
}

// Register adds an engine; a second engine with the same name is a typed
// error, not a silent replacement.
func (r *Registry) Register(e Engine) error {
	name := e.Name()
	if _, dup := r.engines[name]; dup {
		return &DuplicateEngineError{Name: name}
	}
	r.engines[name] = e
	return nil
}

// Engines returns every registered engine in name order.
func (r *Registry) Engines() []Engine {
	engines := make([]Engine, 0, len(r.engines))
	for _, e := range r.engines {
		engines = append(engines, e)
	}
	sort.Slice(engines, func(i, j int) bool { return engines[i].Name() < engines[j].Name() })
	return engines
}

// Status is an engine as the registry lists it: for one that needs a process,
// the process found or the typed error for its absence.
type Status struct {
	Engine string
	// Process names the process found; empty for an in-process engine or an absent one.
	Process string
	// Err is the absence of the process, nil when the engine can run.
	Err error
}

// Statuses lists every engine in name order with the state of its process.
func (r *Registry) Statuses() []Status {
	engines := r.Engines()
	statuses := make([]Status, len(engines))
	for i, e := range engines {
		statuses[i] = Status{Engine: e.Name()}
		if external, ok := e.(External); ok {
			statuses[i].Process, statuses[i].Err = external.Process()
		}
	}
	return statuses
}

// Default returns a registry of every engine the build knows: run, explore, check, sweep
// and solve. Solve registers whether or not a solver is found and refuses through Covers.
func Default() *Registry {
	r := NewRegistry()
	// The five names are distinct constants, so none of these registrations can be refused.
	for _, e := range []Engine{NewRun(), NewExplore(), NewCheck(), NewSweep(), NewSolve(nil)} {
		r.engines[e.Name()] = e
	}
	return r
}

// DefaultFromEnv returns the default registry with every entry of the manifests
// OPENSYSML_TOOLS and OPENSYSML_ENGINES name: one `tool:<name>` engine per tool entry and
// one engine per engine, policy or sampler entry. A manifest that cannot be read, or lies
// under one of the workspaces, is a ManifestError. Each registers whether or not its
// executable is found or this build serves it, and refuses through Covers; a name registered
// twice across the manifests is a DuplicateEngineError.
func DefaultFromEnv(workspaces ...string) (*Registry, error) {
	r := Default()
	externals, err := ExternalsFromEnv(workspaces...)
	if err != nil {
		return nil, err
	}
	for _, e := range externals {
		if err := r.Register(e); err != nil {
			return nil, err
		}
	}
	return r, nil
}
