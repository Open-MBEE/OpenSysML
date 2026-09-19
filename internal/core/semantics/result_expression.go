package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ResultExpression is a result expression a function or expression owns
// (KerML 7.4.7): the trailing expression of a calculation body or the bare
// condition of a constraint body.
type ResultExpression struct {
	Owner *symbols.Symbol
	Node  ast.Node
	// Condition marks a constraint body's condition, one of the several a body
	// may list; a calculation body's expression is a result on its own.
	Condition bool
}

// ResultExpressionConflict is a second result expression membership of a
// function or expression (KerML 8.3.4.6, 8.3.4.8), which allows at most one.
type ResultExpressionConflict struct {
	// Node anchors the fault: the body stated beyond one — a second in the
	// same body or one over an inherited result — or the declaration inheriting two.
	Node ast.Node
	// Stated counts the memberships the type's own body states; the rest are inherited.
	Stated int
	// Owners are the types whose bodies state a membership, one per membership.
	Owners []*symbols.Symbol
}

// FunctionLike reports whether sym declares a function or an expression, the
// types whose bodies state a result expression.
func FunctionLike(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if _, ok := ast.OwnedConstraintOf(sym.Decl); ok {
		return true
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		switch d.Kind {
		case ast.DefCalc, ast.DefConstraint, ast.DefRequirement, ast.DefConcern, ast.DefViewpoint,
			ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase,
			ast.DefPredicate, ast.DefBool:
			return true
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageCalc, ast.UsageExpr, ast.UsageConstraint, ast.UsageRequirement,
			ast.UsageConcern, ast.UsageViewpoint, ast.UsageFramedConcern, ast.UsageSatisfy, ast.UsageObjective,
			ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase,
			ast.UsagePredicate, ast.UsageBool:
			return true
		}
	}
	return false
}

// IsResultExpression reports whether a body member states its owner's result:
// a bare expression, or the bare condition of a constraint body. A nested
// assertion is a constraint of its own, not its owner's result.
func IsResultExpression(node ast.Node) bool {
	if c, ok := node.(*ast.ConstraintMember); ok {
		return c.Keyword == "" && c.Expression != nil
	}
	return ast.IsExpression(node)
}

// OwnedResultExpressions lists the result expressions sym's own body states,
// in declaration order.
func OwnedResultExpressions(sym *symbols.Symbol) []ResultExpression {
	if !FunctionLike(sym) {
		return nil
	}
	var out []ResultExpression
	for _, member := range declMembers(sym) {
		if m, ok := member.(*ast.Membership); ok {
			member = m.Member
		}
		if !IsResultExpression(member) {
			continue
		}
		expr := ResultExpression{Owner: sym, Node: member}
		if c, ok := member.(*ast.ConstraintMember); ok {
			expr.Node, expr.Condition = c.Expression, true
		}
		out = append(out, expr)
	}
	return out
}

// ResultExpressionsOf lists the result expressions sym owns or inherits, its
// own first and then those of every type contributing members to it —
// specialized or reference-subsetted — in breadth-first order (KerML 8.3.4.6
// Expression::result, 8.3.4.8 Function::result). Each source contributes once,
// however many paths reach it; a body listing several conditions contributes each.
func (m *Model) ResultExpressionsOf(sym *symbols.Symbol) []ResultExpression {
	if m == nil || !FunctionLike(sym) {
		return nil
	}
	out := OwnedResultExpressions(sym)
	for _, source := range m.MemberSources(sym) {
		out = append(out, OwnedResultExpressions(source)...)
	}
	return out
}

// ResultExpressionMemberships lists the result expressions sym owns or inherits
// as the memberships KerML counts, sym's own first: each expression of a
// calculation body is one, the conditions a constraint body lists are one
// together. A valid function or expression has at most one.
func (m *Model) ResultExpressionMemberships(sym *symbols.Symbol) []ResultExpression {
	var out []ResultExpression
	for _, expr := range m.ResultExpressionsOf(sym) {
		if expr.Condition && len(out) > 0 {
			if last := out[len(out)-1]; last.Condition && last.Owner == expr.Owner {
				continue
			}
		}
		out = append(out, expr)
	}
	return out
}

// ResultExpressionConflict describes sym's result expression memberships when
// it has more than one; nil when it has at most one.
func (m *Model) ResultExpressionConflict(sym *symbols.Symbol) *ResultExpressionConflict {
	memberships := m.ResultExpressionMemberships(sym)
	if len(memberships) < 2 {
		return nil
	}
	c := &ResultExpressionConflict{Node: sym.Decl, Owners: make([]*symbols.Symbol, len(memberships))}
	for i, expr := range memberships {
		c.Owners[i] = expr.Owner
		if expr.Owner == sym {
			c.Stated++
		}
	}
	switch {
	case c.Stated > 1:
		c.Node = memberships[1].Node
	case c.Stated == 1:
		c.Node = memberships[0].Node
	}
	return c
}
