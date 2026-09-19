package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Codes and messages of the pilot's three operator-expression rules
// (validateOperatorExpression{BracketOperator,Quantity,CastConformance}).
const (
	codeBracketOperator = "bracket-operator"
	codeQuantityUnit    = "quantity-unit"
	codeCastConformance = "cast-conformance"

	msgBracketOperator = "`x[i]` is not an index in KerML: `[` invokes BaseFunctions::'[', which the kernel library leaves abstract; index a sequence with `x#(i)`"
	msgQuantityUnit    = "the unit of a quantity must be a measurement reference, found %s: write a unit such as `[m]` or name a feature typed by MeasurementUnit or another measurement reference"
	msgCastConformance = "cast argument is typed by %s, unrelated to the target %s: neither type specializes the other, so the cast selects no value"
)

// checkOperatorRules applies the three rules to every operator of an expression
// the scalar type checker does not walk: a filter condition or a multiplicity bound.
func (ec *exprChecker) checkOperatorRules(scope *symbols.Scope, node ast.Node) {
	switch e := node.(type) {
	case nil:
		return
	case *ast.OperatorExpr:
		if e.Operator == ast.OpAs {
			ec.checkCast(scope, e)
		}
		for _, o := range e.Operands {
			ec.checkOperatorRules(scope, o)
		}
	case *ast.IndexExpr:
		if e.Bracket {
			ec.checkBracket(scope, e)
		}
		ec.checkOperatorRules(scope, e.Operand)
		ec.checkOperatorRules(scope, e.Index)
	case *ast.FeatureChainExpr:
		ec.checkOperatorRules(scope, e.Operand)
	case *ast.InvocationExpr:
		ec.checkOperatorRules(scope, e.Operand)
		for _, a := range e.Args {
			ec.checkOperatorRules(scope, a)
		}
		for _, na := range e.NamedArgs {
			ec.checkOperatorRules(scope, na.Value)
		}
	case *ast.ConstructorExpr:
		for _, a := range e.Args {
			ec.checkOperatorRules(scope, a)
		}
		for _, na := range e.NamedArgs {
			ec.checkOperatorRules(scope, na.Value)
		}
	case *ast.SequenceExpr:
		for _, el := range e.Elements {
			ec.checkOperatorRules(scope, el)
		}
	case *ast.CollectExpr:
		ec.checkOperatorRules(scope, e.Operand)
		ec.checkOperatorRules(scope, e.Body)
	case *ast.SelectExpr:
		ec.checkOperatorRules(scope, e.Operand)
		ec.checkOperatorRules(scope, e.Body)
	case *ast.BodyExpr:
		for i := range e.Params {
			ec.checkOperatorRules(scope, e.Params[i].Value)
		}
		ec.checkBodyMembers(scope, e)
		ec.checkOperatorRules(ec.bodyScope(scope, e), e.Result)
	case *ast.CastExpr:
		ec.checkBoundOperators(scope, e.Multiplicity)
	}
}

// checkBoundOperators applies the operator rules to the bounds a multiplicity writes.
func (ec *exprChecker) checkBoundOperators(scope *symbols.Scope, mult *ast.Multiplicity) {
	if mult == nil {
		return
	}
	ec.checkOperatorRules(scope, mult.Lower)
	if mult.IsRange {
		ec.checkOperatorRules(scope, mult.Upper)
	}
}

// checkRelationshipBounds applies the operator rules to the end multiplicities
// relationships write, the `[0..1]` of `bind [0..1] a = b`.
func (ec *exprChecker) checkRelationshipBounds(scope *symbols.Scope, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel != nil {
			ec.checkBoundOperators(scope, rel.Multiplicity)
		}
	}
}

// checkUsageBounds applies the operator rules to every multiplicity a usage
// writes: its own, its relationships', its cross feature's and its ends'.
func (ec *exprChecker) checkUsageBounds(scope *symbols.Scope, u *ast.Usage) {
	ec.checkBoundOperators(scope, u.Multiplicity)
	ec.checkRelationshipBounds(scope, u.Relationships)
	inner := childScopeOr(scope, u)
	if cross := u.CrossFeature; cross != nil {
		ec.checkBoundOperators(inner, cross.Multiplicity)
		ec.checkRelationshipBounds(inner, cross.Relationships)
	}
	for _, end := range u.ConnectorEnds {
		if end != nil {
			ec.checkBoundOperators(inner, end.Multiplicity)
			ec.checkRelationshipBounds(inner, end.Relationships)
		}
	}
	if u.FlowEnds != nil {
		ec.checkBoundOperators(inner, u.FlowEnds.PayloadMultiplicity)
	}
}

// checkMemberOperators applies the operator rules to the values and bounds the
// declarations of an expression body write, for a checker that types nothing else of them.
func (ec *exprChecker) checkMemberOperators(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		m = unwrapType(m)
		if d, ok := featureDeclOf(m); ok {
			ec.checkOperatorRules(scope, d.value)
			if u, ok := m.(*ast.Usage); ok {
				ec.checkUsageBounds(scope, u)
			} else {
				ec.checkBoundOperators(scope, d.multiplicity)
			}
		}
		var nested []ast.Node
		switch n := m.(type) {
		case *ast.Usage:
			nested = n.Members
		case *ast.Definition:
			ec.checkBoundOperators(scope, n.Multiplicity)
			ec.checkRelationshipBounds(scope, n.Relationships)
			nested = n.Members
		case *ast.MultiplicityDecl:
			ec.checkBoundOperators(scope, n.Range)
			nested = n.Members
		case *ast.RelationshipMember:
			nested = n.Members
		case *ast.Package:
			nested = n.Members
		case *ast.Namespace:
			nested = n.Members
		}
		if child := childScopeOf(scope, m); child != nil {
			ec.checkMemberOperators(child, nested)
		}
	}
}
