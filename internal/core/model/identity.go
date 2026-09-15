package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// IdentityOf returns the identity side-table entry of a declaration of the named
// document, a bundled library file included; ok is false when it has no
// qualified name or is not indexed there.
func (w *Workspace) IdentityOf(name string, sym *symbols.Symbol) (*identity.Info, bool) {
	if sym == nil || sym.Decl == nil {
		return nil, false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	// The annotation model is keyed by the index's symbol for the same AST node.
	// A library document is parsed afresh from the bytes the index read, so its
	// symbols share the index's spans rather than its nodes.
	same := func(candidate *symbols.Symbol) bool { return candidate.Decl == sym.Decl }
	idx := w.index
	if library, ok := w.libraryNameLocked(name); ok {
		name = library
		same = func(candidate *symbols.Symbol) bool { return candidate.DeclSpan == sym.DeclSpan }
		// A bundled file a version displaced keeps its identity in the library it came from.
		if _, displaced := w.displaced[library]; displaced {
			idx = w.libBase
		}
	}
	var indexed *symbols.Symbol
	walkScope(idx.DocumentRoot(name), func(candidate *symbols.Symbol) {
		if indexed == nil && candidate.Name == sym.Name && same(candidate) {
			indexed = candidate
		}
	})
	if indexed == nil {
		return nil, false
	}
	resolver, sem := w.resolverOver(idx)
	return identity.Of(sem, resolver, indexed)
}
