package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// KerMLValidator's validateFeatureIsVariable message.
const msgVariableFeatureOwner = "Must be owned by an occurrence type"

// KerMLValidator's validateFeaturePortionNotVariable message.
const msgPortionFeatureVariable = "A portion cannot be variable"

// KerMLValidator's validateFeatureValueIsInitial message.
const msgInitialValueNotVariable = "Initialized feature must be variable"

// KerMLValidator's validateFeatureConstantIsVariable message.
const msgConstantNotVariable = "Only a variable feature can be constant"

// VariableFeaturePass checks feature variability (KerML 8.3.3.1.5): a `var` feature is owned
// by an occurrence type and is no portion; only a variable feature is initialized or constant.
type VariableFeaturePass struct{}

func (VariableFeaturePass) Level() PassLevel { return LevelConstraint }

// ElementScoped: each feature gates on its own head and its owner's.
func (VariableFeaturePass) ElementScoped() { /* marker: per-element gating */ }

func (VariableFeaturePass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	model := ctx.Model()
	occurrence := w8cLibraryType(ctx, "Occurrences::Occurrence")
	// A SysML usage's variability is derived from the occurrence library; KerML declares it.
	derivable := ctx.Kind == source.KindKerML
	for _, sym := range ctx.Index.LookupQualified("Occurrences::Occurrence") {
		derivable = derivable || ctx.Index.Library(sym)
	}
	var diags []diag.Diagnostic
	report := func(span source.Span, message, code string) {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     span,
			Message:  message,
			Code:     code,
			Source:   "constraint",
		})
	}
	check := &variableFeatureCheck{
		ctx: ctx, model: model, occurrence: occurrence,
		derivable: derivable, kerml: ctx.Kind == source.KindKerML, report: report,
	}
	w := &kit.Walker{Ctx: ctx}
	w.Walk(rootScope, check.symbol)
	return diags
}

// variableFeatureCheck carries the shared state of one Run's per-symbol checks.
type variableFeatureCheck struct {
	ctx        *Context
	model      *semantics.Model
	occurrence *symbols.Symbol
	derivable  bool
	kerml      bool
	report     func(span source.Span, message, code string)
}

// symbol checks one declaration's variability constraints.
func (c *variableFeatureCheck) symbol(sym *symbols.Symbol) {
	if cross, ok := sym.Decl.(*ast.CrossFeatureMember); ok {
		c.crossFeature(sym, cross)
		return
	}
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || w8cVariabilityDownstream(c.ctx, sym, u) {
		return
	}
	c.usage(sym, u)
}

// crossFeature checks a cross-feature member: a variable one is no portion and,
// its membership being no FeatureMembership, has no owning type.
func (c *variableFeatureCheck) crossFeature(sym *symbols.Symbol, cross *ast.CrossFeatureMember) {
	if c.ctx.DownstreamSpan(cross.Span()) {
		return
	}
	if c.derivable && cross.IsConstant && !c.model.FeatureIsVariable(sym) {
		c.report(cross.Span(), msgConstantNotVariable, "constant-feature-not-variable")
	}
	if !cross.IsVariable && !(cross.IsConstant && c.kerml) {
		return
	}
	if cross.IsPortion {
		c.report(cross.Span(), msgPortionFeatureVariable, "feature-portion-not-variable")
	}
	c.report(cross.Span(), msgVariableFeatureOwner, "variable-feature-owner")
}

// usage checks a usage's variability: `var` wants an occurrence owner and no
// portion; only a variable feature is initialized or constant.
func (c *variableFeatureCheck) usage(sym *symbols.Symbol, u *ast.Usage) {
	if c.derivable && u.ValueIsInitial && u.Value != nil && !c.model.FeatureIsVariable(sym) {
		c.report(w8cValueSpan(u), msgInitialValueNotVariable, "initial-value-not-variable")
	}
	if c.derivable && u.IsConstant && !c.model.FeatureIsVariable(sym) {
		c.report(u.Span(), msgConstantNotVariable, "constant-feature-not-variable")
	}
	// KerML `const` declares a variable feature too; SysML's `constant` does not.
	if !u.IsVariable && !(u.IsConstant && c.kerml) {
		return
	}
	if u.IsPortion {
		c.report(u.Span(), msgPortionFeatureVariable, "feature-portion-not-variable")
	}
	if c.occurrence == nil || sym.OwnerScope == nil {
		return
	}
	owner := sym.OwnerScope.Owner()
	if owner != nil && c.model.Conforms(owner, c.occurrence) {
		return
	}
	c.report(u.Span(), msgVariableFeatureOwner, "variable-feature-owner")
}

// w8cVariabilityDownstream reports a lower-tier failure in what u's variability
// rests on: its own head before the value, or its owner's typing head in this document.
func w8cVariabilityDownstream(ctx *Context, sym *symbols.Symbol, u *ast.Usage) bool {
	if ctx.DownstreamSpan(w8cUsageHead(u)) {
		return true
	}
	if sym.OwnerScope == nil {
		return false
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || owner.Decl == nil {
		return false
	}
	ownerHead, typed := w8cOwnerHead(owner.Decl)
	if !typed {
		return false
	}
	doc := symbols.DocNameOf(owner.OwnerScope)
	if doc == "" {
		doc = owner.DocName
	}
	return (doc == "" || doc == ctx.Name) && ctx.DownstreamSpan(ownerHead)
}

// w8cOwnerHead is the owner's head before its first body member, which fixes its type.
// typed is false for owners whose notation alone fixes it (package, namespace, state node).
func w8cOwnerHead(node ast.Node) (head source.Span, typed bool) {
	switch d := node.(type) {
	case *ast.Usage:
		return w8cUsageHead(d), true
	case *ast.Definition:
		return w8cHeadBefore(d, d.Members), true
	case *ast.SubjectMember:
		return w8cHeadBefore(d, d.Body), true
	case *ast.PrefixMetadata:
		return w8cHeadBefore(d, d.Body), true
	case *ast.MultiplicityDecl:
		return w8cHeadBefore(d, d.Members), true
	case *ast.AssumeMember, *ast.RequireMember:
		if oc, ok := ast.OwnedConstraintOf(d); ok {
			return w8cHeadBefore(d, oc.Body), true
		}
	}
	return source.Span{}, false
}

// w8cUsageHead is a usage's head before its value and its first body member.
func w8cUsageHead(u *ast.Usage) source.Span {
	head := w8cHeadBefore(u, u.Members)
	if u.Value != nil {
		if at := u.Value.Span().Offset; at > head.Offset && at < head.End() {
			head.Len = at - head.Offset
		}
	}
	return head
}

// w8cHeadBefore is a declaration's span before the first of its body members.
func w8cHeadBefore(node ast.Node, body []ast.Node) source.Span {
	span := node.Span()
	if len(body) > 0 {
		if at := body[0].Span().Offset; at > span.Offset && at < span.End() {
			span.Len = at - span.Offset
		}
	}
	return span
}

// w8cValueSpan is the span of a usage's feature value, operator through expression.
func w8cValueSpan(u *ast.Usage) source.Span {
	end := u.Value.Span().End()
	if u.ValueOperatorSpan.Len == 0 || end < u.ValueOperatorSpan.Offset {
		return u.Value.Span()
	}
	return source.Span{Offset: u.ValueOperatorSpan.Offset, Len: end - u.ValueOperatorSpan.Offset}
}

// w8cLibraryType returns the single library symbol for fqn, or nil.
func w8cLibraryType(ctx *Context, fqn string) *symbols.Symbol {
	if ctx == nil || ctx.Index == nil {
		return nil
	}
	syms := ctx.Index.LookupQualified(fqn)
	if len(syms) != 1 {
		return nil
	}
	return syms[0]
}
