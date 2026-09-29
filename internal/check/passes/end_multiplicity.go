package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// msgEndFeatureMultiplicity reports an end feature whose multiplicity is not
// exactly one (KerML validateFeatureEndFeatureMultiplicity).
const msgEndFeatureMultiplicity = "End feature must have multiplicity 1: an end relates exactly one thing per link; write `[1]` or take it from a feature the end subsets or redefines"

// checkFeatureEndFeatureMultiplicity warns on an end feature none of whose
// multiplicities, its own or a general's, is exactly one.
func (cc *constraintChecker) checkFeatureEndFeatureMultiplicity(sym *symbols.Symbol) {
	switch d := sym.Decl.(type) {
	case *ast.ConnectorEnd:
		if _, declares := d.DeclaredName(); !declares || cc.model.EndMultiplicityIsOne(sym) {
			return
		}
		cc.reportEndMultiplicity(d.Span())
	case *ast.Usage:
		if d.IsEnd && !cc.model.EndMultiplicityIsOne(sym) {
			cc.reportEndMultiplicity(d.Span())
		}
		for i, end := range d.ConnectorEnds {
			if end == nil {
				continue
			}
			if _, declares := end.DeclaredName(); declares || cc.model.ConnectorEndMultiplicityIsOne(sym, i) {
				continue
			}
			cc.reportEndMultiplicity(end.Span())
		}
	}
}

func (cc *constraintChecker) checkActionSuccessionSourceMultiplicity(sym *symbols.Symbol) {
	if sym == nil || cc.model == nil || !semantics.ActionDeclaration(sym.Decl) {
		return
	}
	for _, succession := range cc.model.ActionSuccessions(sym) {
		edge, shorthand := succession.Decl.(*ast.SuccessionEdge)
		if succession.Owner != sym || !shorthand {
			continue
		}
		cc.checkActionSuccessionSourceMultiplicityRange(sym.Scope, edge.SourceMultiplicity)
	}
	members := ast.DeclMembers(sym.Decl)
	if members == nil {
		members = ast.NodeBodyMembers(sym.Decl)
	}
	cc.checkNestedActionBodySourceMultiplicities(sym.Scope, members, false)
}

func (cc *constraintChecker) checkNestedActionBodySourceMultiplicities(scope *symbols.Scope, members []ast.Node, checkSuccessions bool) {
	for _, member := range members {
		switch n := kit.UnwrapMembership(member).(type) {
		case *ast.SuccessionEdge:
			if checkSuccessions {
				cc.checkActionSuccessionSourceMultiplicityRange(scope, n.SourceMultiplicity)
			}
			if len(n.Members) > 0 {
				cc.checkNestedActionBodySourceMultiplicities(kit.BodyScope(scope, n), n.Members, true)
			}
		case *ast.IfActionNode:
			for _, branch := range n.Branches() {
				cc.checkNestedActionBodySourceMultiplicities(kit.BodyScope(scope, branch), branch.Body, true)
			}
		case *ast.IfBranchNode:
			cc.checkNestedActionBodySourceMultiplicities(kit.BodyScope(scope, n), n.Body, true)
		case *ast.WhileLoopActionNode:
			cc.checkNestedActionBodySourceMultiplicities(kit.BodyScope(scope, n), n.Body, true)
		}
	}
}

func (cc *constraintChecker) checkActionSuccessionSourceMultiplicityRange(scope *symbols.Scope, multiplicity *ast.Multiplicity) {
	if multiplicity == nil {
		return
	}
	r, ok := cc.model.RangeIn(scope, multiplicity)
	if !ok {
		return
	}
	if value, exact := r.Exactly(); exact && value == 1 {
		return
	}
	cc.reportEndMultiplicity(multiplicity.Span())
}

func (cc *constraintChecker) reportEndMultiplicity(span source.Span) {
	cc.diags = append(cc.diags, diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     span,
		Message:  msgEndFeatureMultiplicity,
		Code:     "end-feature-multiplicity",
		Source:   "constraint",
	})
}
