package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// KerML type-relationship constraint messages, quoted from KerMLValidator.
const (
	msgOnlyOneUnioning      = "Cannot have only one unioning"
	msgOnlyOneIntersecting  = "Cannot have only one intersecting"
	msgOnlyOneDifferencing  = "Cannot have only one differencing"
	msgUnioningSelf         = "Type cannot union with itself"
	msgIntersectingSelf     = "Type cannot intersect with itself"
	msgDifferencingSelf     = "Type cannot difference with itself"
	msgOnlyOneChaining      = "Cannot have only one chaining feature"
	msgChainingFeaturesSelf = "Feature cannot have itself in a feature chain"
)

// TypeRelationshipsPass checks the union, intersection, difference and chaining
// relationships a type owns for a single operand or a self operand (KerML
// 8.3.3.1.2, 8.3.3.1.5). Constraint tier: the self rules resolve targets.
type TypeRelationshipsPass struct{}

func (TypeRelationshipsPass) Level() PassLevel { return LevelConstraint }

func (TypeRelationshipsPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	w := &kit.Walker{Ctx: ctx}
	c := &typeRelationshipsChecker{resolver: ctx.Resolver()}
	w.Walk(rootScope, c.check)
	return c.diags
}

type typeRelationshipsChecker struct {
	resolver *resolve.Resolver
	diags    []diag.Diagnostic
}

func (c *typeRelationshipsChecker) check(sym *symbols.Symbol) {
	c.checkNotOneAndSelf(sym, ast.RelUnions, msgOnlyOneUnioning, msgUnioningSelf, "type-unioning")
	c.checkNotOneAndSelf(sym, ast.RelIntersects, msgOnlyOneIntersecting, msgIntersectingSelf, "type-intersecting")
	c.checkNotOneAndSelf(sym, ast.RelDifferences, msgOnlyOneDifferencing, msgDifferencingSelf, "type-differencing")
	c.checkChaining(sym)
}

// checkNotOneAndSelf reports a type owning exactly one relationship of kind (a
// one-operand union is the operand), and one whose target is the owner itself.
func (c *typeRelationshipsChecker) checkNotOneAndSelf(sym *symbols.Symbol, kind ast.RelationshipKind, notOne, self, code string) {
	var rels []*ast.Relationship
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel != nil && rel.Kind == kind && rel.Target != nil {
			rels = append(rels, rel)
		}
	}
	if len(rels) == 0 {
		return
	}
	if len(rels) == 1 {
		c.report(rels[0].Target.Span(), notOne, code+"-not-one")
	}
	for _, rel := range rels {
		target, ok := c.resolver.ResolveTarget(kit.DeclarationScope(sym), rel.Target)
		if ok && target == sym {
			// The rule is about the type, so it is reported on the type, not on
			// the operand naming it.
			c.report(w8cNameSpan(sym), self, code+"-self")
		}
	}
}

// checkChaining reports a feature whose chain names a single feature, and one
// that names itself among its chaining features.
func (c *typeRelationshipsChecker) checkChaining(sym *symbols.Symbol) {
	var steps []kit.ChainStep
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel != nil && rel.Kind == ast.RelChains && rel.Target != nil {
			steps = append(steps, kit.ChainSteps(rel.Target)...)
		}
	}
	if len(steps) == 0 {
		return
	}
	if len(steps) == 1 {
		c.report(steps[0].Span, msgOnlyOneChaining, "chaining-feature-not-one")
	}
	for _, step := range steps {
		target, ok := c.resolver.ResolveTarget(kit.DeclarationScope(sym), step.Node)
		if ok && target == sym {
			c.report(w8cNameSpan(sym), msgChainingFeaturesSelf, "chaining-features-not-self")
		}
	}
}

func (c *typeRelationshipsChecker) report(span source.Span, msg, code string) {
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     span,
		Message:  msg,
		Code:     code,
		Source:   "constraint",
	})
}

// w8cNameSpan is the span of a declaration's identifier, falling back to the
// whole declaration when the identifier is not recorded.
func w8cNameSpan(sym *symbols.Symbol) source.Span {
	if sym.NameSpan != (source.Span{}) {
		return sym.NameSpan
	}
	return sym.DeclSpan
}
