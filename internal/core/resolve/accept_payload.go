package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// acceptPayload resolves name as the payload an accept node declared in scope
// binds: the nodes of one body share a feature space (SysML v2 7.16.5), and only
// that scope contributes it, so a nearer declaration wins (KerML 8.2.3.5.3) and
// the payload does not escape the body.
func (r *Resolver) acceptPayload(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	if scope == nil || name == "" {
		return nil, false
	}
	payloads, done := r.payloads[scope]
	if !done {
		payloads = acceptPayloadsIn(scope)
		r.payloads[scope] = payloads
	}
	sym, ok := payloads[name]
	return sym, ok
}

// acceptPayloadsIn keys the payloads bound by the accept nodes declared directly
// in scope by name; the first of two accepts binding one name keeps it.
func acceptPayloadsIn(scope *symbols.Scope) map[string]*symbols.Symbol {
	if !sharesBodyFeatureSpace(scope.Node()) {
		return nil
	}
	var payloads map[string]*symbols.Symbol
	for _, node := range scope.Children() {
		usage, ok := node.Node().(*ast.Usage)
		if !ok || usage.Kind != ast.UsageAction {
			continue
		}
		for _, member := range usage.Members {
			decl, _ := unwrapForResolve(member)
			name, ok := acceptPayloadName(decl)
			if !ok {
				continue
			}
			sym, ok := node.LookupLocal(name)
			if !ok {
				continue
			}
			if _, taken := payloads[name]; taken {
				continue
			}
			if payloads == nil {
				payloads = map[string]*symbols.Symbol{}
			}
			payloads[name] = sym
		}
	}
	return payloads
}

// sharesBodyFeatureSpace reports whether node is an action body, whose nodes
// execute against one feature space — a part declaring an action node is not one.
// A state body's accept is a transition trigger, scoped by symbols.TriggerScope.
func sharesBodyFeatureSpace(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.Usage:
		return n.Kind == ast.UsageAction
	case *ast.Definition:
		return n.Kind == ast.DefAction
	case *ast.IfBranchNode, *ast.WhileLoopActionNode:
		return true
	}
	return false
}

// acceptPayloadName returns the name a node's accept member binds its payload
// under; a trigger expression (`accept when x > 1`) declares no payload. The name
// is the declared one, as lower.Accept.ParamName is: a payload naming itself
// after a referenced feature (`accept ::> shutDown`) binds nothing at execution,
// so it must not mask that feature here either.
func acceptPayloadName(decl ast.Node) (string, bool) {
	payload, ok := decl.(*ast.Usage)
	if !ok || !payload.IsAccept || payload.Value != nil {
		return "", false
	}
	return payload.Ident.Name, payload.Ident.Name != ""
}
