package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const msgW9CBoundFeatureTypes = "Bound features should have conforming types"

// W9CBoundFeatureTypesPass checks that the two features a binding connector
// binds have conforming types (SysML 8.3.3.1, KerML 8.3.4.3): binding features
// of unrelated types cannot be satisfied. Besides `bind a = b` it judges the
// bindings the language implies: a function's result expression to its result
// parameter, a subject's value or the subject of a `satisfy … by` to the subject
// it fills, and a nested requirement's subject to its owner's. The binding an
// operator or invocation argument implies is judged by the type checker, which
// knows the parameter it fills (w9c_argument_bindings.go).
type W9CBoundFeatureTypesPass struct{}

func (W9CBoundFeatureTypesPass) Level() PassLevel { return LevelType }

func (W9CBoundFeatureTypesPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &w9cBindingChecker{model: ctx.Model(), resolver: ctx.Resolver(), idx: ctx.Index}
	if c.model == nil || c.resolver == nil {
		return nil
	}
	w8dWalkSymbols(ctx, rootScope, c.check)
	return c.diags
}

type w9cBindingChecker struct {
	model    *semantics.Model
	resolver *resolve.Resolver
	idx      *symbols.Index
	diags    []Diagnostic
}

func (c *w9cBindingChecker) check(sym *symbols.Symbol) {
	if c.idx.Library(sym) {
		return
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		if d.Kind == ast.DefCalc {
			c.checkResultExpressions(sym, d.Span())
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageBinding:
			c.checkBinding(sym, d)
		case ast.UsageCalc, ast.UsageExpr:
			c.checkResultExpressions(sym, d.Span())
		case ast.UsageSatisfy:
			c.checkSatisfySubject(sym, d)
		case ast.UsageSubject:
			c.checkSubject(sym, d.Value, d.ValueIsDefault, d.Span())
		}
	case *ast.SubjectMember:
		c.checkSubject(sym, d.BindingExpr, d.ValueIsDefault, d.Span())
	}
}

func (c *w9cBindingChecker) checkBinding(sym *symbols.Symbol, u *ast.Usage) {
	ends := c.bindingEnds(sym, u)
	if len(ends) != 2 {
		return
	}
	left, right := c.endTypes(ends[0]), c.endTypes(ends[1])
	if len(left) == 0 || len(right) == 0 || w9cTypesConform(c.model, left, right) ||
		c.endConforms(ends[0], right) || c.endConforms(ends[1], left) {
		return
	}
	c.report(u.Span())
}

// checkResultExpressions judges the binding of a function's result expressions
// to its result parameter (KerML 8.4.4.7).
func (c *w9cBindingChecker) checkResultExpressions(sym *symbols.Symbol, span source.Span) {
	want := c.model.FeatureTypeSet(c.model.ResultParameterOf(sym))
	if len(want) == 0 {
		return
	}
	for _, expr := range semantics.OwnedResultExpressions(sym) {
		if !c.valueConforms(sym.Scope, expr.Node, want) {
			c.report(span)
			return
		}
	}
}

// checkSubject judges a subject parameter against what fills it: the value it
// is written with, else the subject of the requirement or case its owner nests
// in, which the owner's subject implicitly binds to (SysML 8.3.19.7).
func (c *w9cBindingChecker) checkSubject(sym *symbols.Symbol, value ast.Node, isDefault bool, span source.Span) {
	want := c.model.FeatureTypeSet(sym)
	if len(want) == 0 {
		return
	}
	if value != nil {
		if !isDefault && !c.valueConforms(w8cScopeOf(sym), value, want) {
			c.report(span)
		}
		return
	}
	outer := c.enclosingSubject(sym)
	if outer == nil {
		return
	}
	got := c.model.FeatureTypeSet(outer)
	if len(got) == 0 || w9cTypesConform(c.model, got, want) {
		return
	}
	c.report(span)
}

// enclosingSubject is the subject a nested requirement's or case's own subject
// is bound to: that of the requirement or case the non-abstract usage owning
// the subject is itself declared in.
func (c *w9cBindingChecker) enclosingSubject(subject *symbols.Symbol) *symbols.Symbol {
	owner := w9cOwnerOf(subject)
	if owner == nil {
		return nil
	}
	u, ok := owner.Decl.(*ast.Usage)
	if !ok || u.IsAbstract {
		return nil
	}
	enclosing := w9cOwnerOf(owner)
	if enclosing == nil || !w9cSubjectFlowsInto(u.Kind, enclosing) {
		return nil
	}
	return c.model.SubjectParameterOf(enclosing)
}

func w9cOwnerOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// w9cSubjectFlowsInto reports whether a usage of the kind, declared in owner,
// has its subject bound to owner's: a requirement in a requirement, a case in
// a case.
func w9cSubjectFlowsInto(kind ast.UsageKind, owner *symbols.Symbol) bool {
	switch kind {
	case ast.UsageRequirement, ast.UsageSatisfy:
		return w9cIsRequirement(owner)
	case ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
		return w9cIsCase(owner)
	}
	return false
}

func w9cIsRequirement(sym *symbols.Symbol) bool {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefRequirement
	case *ast.Usage:
		return d.Kind == ast.UsageRequirement || d.Kind == ast.UsageSatisfy
	}
	return false
}

func w9cIsCase(sym *symbols.Symbol) bool {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		switch d.Kind {
		case ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
			return true
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
			return true
		}
	}
	return false
}

// checkSatisfySubject judges the `by` operand of a satisfy against the subject
// of the requirement it satisfies, which the operand is bound to.
func (c *w9cBindingChecker) checkSatisfySubject(sym *symbols.Symbol, u *ast.Usage) {
	var by ast.Node
	typed := false
	for _, rel := range u.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubject:
			by = rel.Target
		case ast.RelTyping:
			typed = true
		}
	}
	if by == nil {
		return
	}
	// A satisfy naming a definition rather than a usage is reported elsewhere.
	if requirement, _ := c.model.SatisfyTarget(sym); requirement == nil || !typed && !requirement.IsFeature() {
		return
	}
	want := c.model.FeatureTypeSet(c.model.SubjectParameterOf(sym))
	if len(want) == 0 || c.valueConforms(w8cScopeOf(sym), by, want) {
		return
	}
	c.report(by.Span())
}

// valueConforms reports whether one of a value's result types conforms to one of
// the types of the feature it is bound to, or the reverse; an unknown result
// type is not judged.
func (c *w9cBindingChecker) valueConforms(scope *symbols.Scope, value ast.Node, want []*symbols.Symbol) bool {
	got := c.model.ExprResultTypes(scope, value)
	return len(got) == 0 || w9cTypesConform(c.model, got, want)
}

func (c *w9cBindingChecker) report(span source.Span) {
	c.diags = append(c.diags, w9cDiagnostic(span))
}

// w9cDiagnostic is the warning that two bound features have non-conforming types, at span.
func w9cDiagnostic(span source.Span) Diagnostic {
	return Diagnostic{
		Severity: SeverityWarning,
		Span:     span,
		Message:  msgW9CBoundFeatureTypes,
		Code:     "bound-feature-types",
		Source:   "type",
	}
}

// bindingEnds resolves the two features a `bind a = b;` names.
func (c *w9cBindingChecker) bindingEnds(sym *symbols.Symbol, u *ast.Usage) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, end := range u.ConnectorEnds {
		feature := end.AttachedTarget()
		if feature == nil {
			return nil
		}
		target, ok := c.resolver.ResolveTarget(w8cScopeOf(sym), feature)
		if !ok || target == nil {
			return nil
		}
		out = append(out, target)
	}
	return out
}

// endTypes returns an end's declared types, following a single untyped
// reference subsetting so `ref x ::> y` is judged by y's types.
func (c *w9cBindingChecker) endTypes(end *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range semantics.RelationshipsOf(end) {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		if t, ok := c.resolver.ResolveTarget(w8cScopeOf(end), rel.Target); ok && t != nil {
			out = append(out, t)
		}
	}
	return w8cMostSpecific(c.model, out)
}

// endConforms reports whether the end itself conforms to one of the other side's
// types through an implicit typing: a variant is typed by its variation.
func (c *w9cBindingChecker) endConforms(end *symbols.Symbol, types []*symbols.Symbol) bool {
	for _, t := range types {
		if c.model.Conforms(end, t) {
			return true
		}
	}
	return false
}

// w9cTypesConform reports whether some type on one side conforms to one on the
// other: a binding needs only one compatible pairing.
func w9cTypesConform(model *semantics.Model, left, right []*symbols.Symbol) bool {
	for _, l := range left {
		for _, r := range right {
			if l == r || model.Conforms(l, r) || model.Conforms(r, l) {
				return true
			}
		}
	}
	return false
}
