package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func associationDefinition(sym *symbols.Symbol) bool {
	return sym != nil && sym.Kind == symbols.SymbolConnectionDef
}

func dependencyDot(dep *ast.Dependency) bool {
	return len(dep.Clients) > 1 || len(dep.Suppliers) > 1
}

// rewritesTreeBox shares the eligibility predicates of association and exposed-relationship lowering.
func (r *Renderer) rewritesTreeBox(sym *symbols.Symbol) bool {
	if associationDefinition(sym) {
		return true
	}
	if dep, ok := sym.Decl.(*ast.Dependency); ok && dependencyDot(dep) {
		return true
	}
	return nary(len(r.model.ConnectorEndAttachments(sym)))
}
