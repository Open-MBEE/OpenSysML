package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// DocumentationOf returns the prose of the `doc` members sym owns, in
// declaration order: its `Element::documentation`, as the model reads it.
func (w *Workspace) DocumentationOf(sym *symbols.Symbol) []string {
	if sym == nil || sym.DocName == "" {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	w.queryLocked(sym.DocName, func(_ *resolve.Resolver, sem *semantics.Model) {
		out = sem.DocumentationOf(sym)
	})
	return out
}
