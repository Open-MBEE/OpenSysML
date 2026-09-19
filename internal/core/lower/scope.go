package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// childScope returns the scope decl owns under parent — the namespace a nested
// action node, a loop body, an `if` branch, a state or a region declares into —
// or parent itself when decl owns none, which is the case for a declaration the
// scope builder does not give a scope of its own.
//
// Lowering resolves scopes this way rather than from a declaration's symbol so
// that a body the parser reshapes on the way in (`state s { … }`, whose members
// become a synthesized StateNode) still gets the scope its source was written
// in: the scope tree is keyed by the declaration the builder saw.
func childScope(parent *symbols.Scope, decl ast.Node) *symbols.Scope {
	if parent == nil || decl == nil {
		return parent
	}
	for _, child := range parent.Children() {
		if child.Node() == decl {
			return child
		}
	}
	return parent
}
