package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// drawnTypes are the types sym is declared with as a diagram draws them: a
// ConjugatedPortDefinition, implied and never drawn, stands as the port definition it conjugates.
func (r *Renderer) drawnTypes(sym *symbols.Symbol) []*symbols.Symbol {
	types := r.model.DeclaredTypes(sym)
	out := make([]*symbols.Symbol, 0, len(types))
	for _, typ := range types {
		if original := semantics.OriginalPortDefinitionOf(typ); original != nil {
			typ = original
		}
		out = append(out, typ)
	}
	return out
}
