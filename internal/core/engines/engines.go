// Package engines assembles the registry of every analysis engine the build knows:
// the framework's own and those of packages the framework cannot import.
package engines

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/smt"
)

// Default returns the build's registry: run, explore, check, sweep, solve and smt.
// Smt registers whether or not a solver is found and refuses through Covers.
func Default() *analysis.Registry {
	r := analysis.Default()
	register(r)
	return r
}

// DefaultFromEnv returns the build's registry with every entry of the manifests
// OPENSYSML_TOOLS and OPENSYSML_ENGINES name, as analysis.DefaultFromEnv reads them; a
// manifest that cannot be read, or lies under one of the workspaces, is a ManifestError.
func DefaultFromEnv(workspaces ...string) (*analysis.Registry, error) {
	r, err := analysis.DefaultFromEnv(workspaces...)
	if err != nil {
		return nil, err
	}
	register(r)
	return r, nil
}

// register adds the engines of other packages; their names are distinct constants
// none of the framework's own carries, so no registration can be refused.
func register(r *analysis.Registry) {
	if err := r.Register(smt.New(nil)); err != nil {
		panic(err)
	}
}
