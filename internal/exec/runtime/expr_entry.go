package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ExprEntry is the expression behind id in the model's table, nil for none: what a
// condition's, an objective's or a declared value's id names.
func (ctx *Context) ExprEntry(id program.ExprID) *lower.ExprEntry {
	if id == 0 {
		return nil
	}
	return ctx.model.exprs.Entry(id)
}

// VariantValueEntry is the value a variant states, in the model's table; nil for none.
func (ctx *Context) VariantValueEntry(variant *symbols.Symbol) *lower.ExprEntry {
	if variant == nil {
		return nil
	}
	return ctx.ExprEntry(ctx.model.exprs.Intern(semantics.VariantValue(variant), variant.OwnerScope))
}

// written is the syntax behind entry, nil for none.
func written(entry *lower.ExprEntry) ast.Node {
	if entry == nil {
		return nil
	}
	return entry.Tree
}

// EvalEntry is Eval of a lowered expression.
func (ec *EvalContext) EvalEntry(entry *lower.ExprEntry) (Value, error) {
	return ec.Eval(written(entry))
}

// EvalEntryOn is EvalWithScopeOn of a lowered expression.
func (ctx *Context) EvalEntryOn(entry *lower.ExprEntry, scope *symbols.Scope, self *Instance) (Value, error) {
	return ctx.EvalWithScopeOn(written(entry), scope, self)
}

// ReadEntryInSatisfaction is ReadInSatisfaction of a lowered reference.
func (ctx *Context) ReadEntryInSatisfaction(a *SatisfyAssertion, subject *Instance, scope *symbols.Scope, ref *lower.ExprEntry) (Value, error) {
	return ctx.ReadInSatisfaction(a, subject, scope, written(ref))
}

// EntryText is ExprText of a lowered expression.
func (ctx *Context) EntryText(entry *lower.ExprEntry, scope *symbols.Scope) string {
	return ctx.ExprText(written(entry), scope)
}

// Written is where the steps begin, empty for none.
func (s *BodySteps) Written() source.Span {
	if s == nil || s.Node == nil {
		return source.Span{}
	}
	return s.Node.Span()
}

// Written is where the condition's own expression was written, false for a condition stating none.
func (c Condition) Written() (source.Span, bool) {
	if c.Expr == nil {
		return source.Span{}, false
	}
	return c.Expr.Span(), true
}

// Written is where the objective's value was written, false for an objective stating none.
func (o Objective) Written() (source.Span, bool) {
	if o.Value == nil {
		return source.Span{}, false
	}
	return o.Value.Span(), true
}

// TokenLabelOf is TokenLabel of the node behind node in graph.
func TokenLabelOf(id int64, graph *lower.ActionGraph, node program.NodeID) string {
	return TokenLabel(id, graph.Provenance.Node(node))
}

// BranchLabelOf is BranchLabel of the target behind target in graph.
func BranchLabelOf(pos int, graph *lower.ActionGraph, target program.NodeID) string {
	return BranchLabel(pos, graph.Provenance.Node(target))
}

// DecisionPlaceOf is DecisionPlace of the decision behind node in graph.
func DecisionPlaceOf(graph *lower.ActionGraph, node program.NodeID) string {
	return DecisionPlace(graph.Provenance.Node(node))
}
