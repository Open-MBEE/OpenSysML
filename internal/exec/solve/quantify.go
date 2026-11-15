package solve

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// quantified translates `P->forAll { in ref w; w == V }` and `P->exists { … }`
// over a variation point P. A configuration holds only the variant P selects
// (SysML v2.1 §8.3.6, §7.6.7), so every value P holds is that variant and the
// quantifier states the selection itself: `P == V` (`P != V` for `w != V`),
// provided P holds at least one object. Every other invocation is refused.
func (t *translator) quantified(n *lower.ExprEntry, scope *symbols.Scope) (*Term, error) {
	name := t.quantifierName(n, scope)
	if !n.Receiver || name == "" {
		return nil, t.refuse(n, describe(n), "it is outside the subset")
	}
	receiver := n.Operands[0]
	construct := "`->" + name + "` expression"
	shape := "only `{ in ref w; w == variant }` over a variation point, comparing the iterate with a variant, is translated"
	if len(n.Operands) != 2 || len(n.Named) != 0 {
		return nil, t.refuse(n, construct, shape)
	}
	body := n.Operands[1]
	if body.Kind != program.ExprBody || len(body.Params) != 1 || body.Locals != 0 {
		return nil, t.refuse(n, construct, shape)
	}
	cmp := body.Answer
	if cmp == nil || cmp.Kind != program.ExprOp || (cmp.Op != program.OpEq && cmp.Op != program.OpNeq) || len(cmp.Operands) != 2 {
		return nil, t.refuse(n, construct, shape)
	}
	var other *lower.ExprEntry
	switch iterate := body.Params[0].Name; {
	case namesParam(cmp.Operands[0], iterate):
		other = cmp.Operands[1]
	case namesParam(cmp.Operands[1], iterate):
		other = cmp.Operands[0]
	default:
		return nil, t.refuse(n, construct, shape)
	}
	segments, ok := receiver.Segments()
	if !ok || len(segments) == 0 {
		return nil, t.refuse(n, construct, "its receiver names nothing this translation can ground")
	}
	chain, err := t.resolvePath(receiver, scope, segments)
	if err != nil {
		return nil, err
	}
	point := chain[len(chain)-1]
	if !t.model.IsVariationFeature(point) {
		return nil, t.refuse(n, construct, "its receiver "+point.Name+" is not a variation point")
	}
	if r := t.model.GoverningMultiplicityOf(point); r.AllowsNone() {
		return nil, t.refuse(n, construct, "its receiver "+point.Name+" may hold no object, so the quantifier says nothing about the selection")
	} else if !r.Lower.Known || r.Lower.Infinite || r.Lower.Value < 1 {
		return nil, t.refuse(n, construct, "the multiplicity of its receiver "+point.Name+" does not state that it holds an object")
	}
	left, err := t.reference(receiver, scope)
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
	if cmp.Op == program.OpNeq {
		op = OpNe
	}
	return t.equalityOf(n, scope, construct, op, left, right)
}

// quantifierName is "forAll" or "exists" when the invoked function resolves to
// the ControlFunctions one of that name, read through the same resolution the
// evaluator invokes by, an alias through its target; "" for any other function.
func (t *translator) quantifierName(n *lower.ExprEntry, scope *symbols.Scope) string {
	if n.Ref.Name == "" {
		return ""
	}
	segments := strings.Split(n.Ref.Name, "::")
	chain, err := t.resolvePath(n, scope, segments)
	if err != nil || len(chain) == 0 {
		return ""
	}
	callee := chain[len(chain)-1]
	if target, ok := t.ctx.Resolver().ResolveAliasTarget(callee); ok && target != nil {
		callee = target
	}
	switch t.fqn(callee) {
	case "ControlFunctions::forAll":
		return "forAll"
	case "ControlFunctions::exists":
		return "exists"
	}
	return ""
}

// namesParam reports whether node is a bare reference to the body parameter name.
func namesParam(node *lower.ExprEntry, name string) bool {
	return node.Kind == program.ExprRef && node.Reference() && len(node.Path) == 0 && node.Ref.Name == name
}
