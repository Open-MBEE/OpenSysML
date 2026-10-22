package solve

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// quantified translates `P->forAll { in ref w; w == V }` and `P->exists { … }`
// over a variation point P. A configuration holds only the variant P selects
// (SysML v2.1 §8.3.6, §7.6.7), so every value P holds is that variant and the
// quantifier states the selection itself: `P == V` (`P != V` for `w != V`),
// provided P holds at least one object. Every other invocation is refused.
func (t *translator) quantified(n *ast.InvocationExpr, scope *symbols.Scope) (*Term, error) {
	name := quantifierName(n.Type)
	if n.Operand == nil || (name != "forAll" && name != "exists") {
		return nil, t.refuse(n, describe(n), "it is outside the subset")
	}
	construct := "`->" + name + "` expression"
	shape := "only `{ in ref w; w == variant }` over a variation point, comparing the iterate with a variant, is translated"
	if len(n.Args) != 1 {
		return nil, t.refuse(n, construct, shape)
	}
	body, ok := n.Args[0].(*ast.BodyExpr)
	if !ok || len(body.Params) != 1 || len(body.Members) != 0 {
		return nil, t.refuse(n, construct, shape)
	}
	cmp, ok := body.Result.(*ast.OperatorExpr)
	if !ok || (cmp.Operator != ast.OpEq && cmp.Operator != ast.OpNeq) || len(cmp.Operands) != 2 {
		return nil, t.refuse(n, construct, shape)
	}
	var other ast.Node
	switch iterate := body.Params[0].Name; {
	case namesParam(cmp.Operands[0], iterate):
		other = cmp.Operands[1]
	case namesParam(cmp.Operands[1], iterate):
		other = cmp.Operands[0]
	default:
		return nil, t.refuse(n, construct, shape)
	}
	segments, ok := pathSegments(n.Operand)
	if !ok || len(segments) == 0 {
		return nil, t.refuse(n, construct, "its receiver names nothing this translation can ground")
	}
	chain, err := t.resolvePath(n.Operand, scope, segments)
	if err != nil {
		return nil, err
	}
	point := chain[len(chain)-1]
	if !t.model.IsVariationFeature(point) {
		return nil, t.refuse(n, construct, "its receiver "+point.Name+" is not a variation point")
	}
	if t.model.GoverningMultiplicityOf(point).AllowsNone() {
		return nil, t.refuse(n, construct, "its receiver "+point.Name+" may hold no object, so the quantifier says nothing about the selection")
	}
	left, err := t.reference(n.Operand, scope)
	if err != nil {
		return nil, err
	}
	right, err := t.expr(other, scope)
	if err != nil {
		return nil, err
	}
	if !left.Sort.Equal(right.Sort) {
		return nil, t.refuse(n, construct, "the iterate is compared with a value of "+right.Sort.Name+", not a variant of "+point.Name)
	}
	op := OpEq
	if cmp.Operator == ast.OpNeq {
		op = OpNe
	}
	return Binary(op, Bool, left, right), nil
}

// quantifierName is the invoked function's own name, `forAll` for `ControlFunctions::forAll`.
func quantifierName(qn *ast.QualifiedName) string {
	if qn == nil || len(qn.Parts) == 0 {
		return ""
	}
	return qn.Parts[len(qn.Parts)-1].Text
}

// namesParam reports whether node is a bare reference to the body parameter name.
func namesParam(node ast.Node, name string) bool {
	ref, ok := node.(*ast.FeatureReference)
	return ok && ref.Name != nil && len(ref.Name.Parts) == 1 && ref.Name.Parts[0].Text == name
}
