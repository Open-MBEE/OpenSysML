package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// typedModel is NewModel over a semantic model with the checker's argument typing
// installed, as every product path builds one, so a run selects calls as validation did.
func typedModel(sem *semantics.Model, resolver *resolve.Resolver) *Model {
	if sem != nil {
		sem.SetArgumentTyper(passes.NewArgumentTyper(resolver, sem))
	}
	return NewModel(sem, resolver)
}
