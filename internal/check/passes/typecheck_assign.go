package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// checkAssignmentValue checks the value an assignment writes against the type
// and multiplicity of the feature it writes it to. An assignment assigns the
// values of the feature it accesses (KerML FeatureWritePerformance), so the
// rule is the one that governs binding an initial value; a value whose type is
// not statically known stays for the run time to judge.
func (ec *exprChecker) checkAssignmentValue(scope *symbols.Scope, n *ast.AssignmentActionNode) {
	if n == nil || n.Value == nil {
		return
	}
	target, ok := ec.resolveAssignmentTarget(scope, n.Target)
	if !ok {
		// No feature the declarations identify, so only the written expression
		// itself can be checked.
		ec.infer(scope, n.Value)
		return
	}
	d, isFeature := featureDeclOf(target.Decl)
	if !isFeature {
		ec.infer(scope, n.Value)
		return
	}
	// The written value's names are the statement's; the declaration read is
	// the target feature's own, which a chained target reaches elsewhere.
	ec.checkBoundValue(scope, target.OwnerScope, d, n.Value, nil)
}

// resolveAssignmentTarget resolves the feature an assignment writes, following
// a feature chain to its final segment: that segment is the feature written.
func (ec *exprChecker) resolveAssignmentTarget(scope *symbols.Scope, target ast.Node) (*symbols.Symbol, bool) {
	if target == nil {
		return nil, false
	}
	sym, ok := ec.resolver.ResolveTarget(scope, target)
	if !ok || sym == nil {
		return nil, false
	}
	if sym.Kind == symbols.SymbolAlias {
		if resolved, ok := ec.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
			sym = resolved
		}
	}
	return sym, true
}
