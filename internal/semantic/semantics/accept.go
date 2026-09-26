package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// AcceptPayload reports whether sym is the payload an accept node binds, which
// the nodes of one action body share (resolve.acceptPayloadsIn), so a sibling
// node's body reaches it under its bare name.
func AcceptPayload(sym *symbols.Symbol) bool {
	if sym == nil || sym.OwnerScope == nil {
		return false
	}
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModAcceptPayload)
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return false
	}
	node, ok := owner.Decl.(*ast.Usage)
	if !ok || node.Kind != ast.UsageAction {
		return false
	}
	for _, member := range node.Members {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		payload, ok := member.(*ast.Usage)
		if ok && payload.IsAccept && payload.Value == nil && payload.Ident.Name == sym.Name {
			return true
		}
	}
	return false
}
