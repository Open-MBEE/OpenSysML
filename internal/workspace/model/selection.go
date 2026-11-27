package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// DeclaredViews returns the views declared in scope and its nested scopes,
// outermost first and in declaration order.
func DeclaredViews(scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	scope.WalkDeclarations(func(sym *symbols.Symbol) {
		if semantics.IsView(sym) {
			out = append(out, sym)
		}
	})
	return out
}
