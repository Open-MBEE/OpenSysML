package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Origin is the source declaration that produced a rendered artifact.
type Origin = symbols.Origin

// symbolOrigin is where a symbol was declared, the zero Origin for a symbol
// carrying no declaration of its own.
func symbolOrigin(sym *symbols.Symbol) Origin {
	return sym.Origin()
}

// nodeOrigin is where an AST node of a lowered graph was written, in the
// document the element that lowered to it was declared in.
func nodeOrigin(doc string, node ast.Node) Origin {
	return symbols.NodeOrigin(doc, node)
}

// inheritedOrigins is where the declarations a lowered graph took content from
// were written, in the graph's order.
func inheritedOrigins(inherited []lower.Inherited) []Origin {
	out := make([]Origin, 0, len(inherited))
	for _, in := range inherited {
		out = append(out, nodeOrigin(symbols.DocNameOf(in.Body), in.Decl))
	}
	return out
}
