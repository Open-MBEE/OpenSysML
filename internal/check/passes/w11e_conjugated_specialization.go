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

// msgConjugatedSpecific is quoted from KerMLValidator.
const msgConjugatedSpecific = "Conjugated type cannot be a specialized type"

// W11EConjugatedSpecializationPass checks that a conjugated type is not the specific type
// of a specialization (KerML 1.0 validateSpecializationSpecificNotConjugated).
// The specialization metaclass rules are W11AKerMLSpecializationPass's, and this
// pass shares its level so a kind error does not suppress this one.
type W11EConjugatedSpecializationPass struct{}

func (W11EConjugatedSpecializationPass) Level() PassLevel { return LevelType }

func (W11EConjugatedSpecializationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil || ctx.Kind != source.KindKerML {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	w := &kit.Walker{Ctx: ctx}
	c := &w11eConjugatedChecker{resolver: ctx.Resolver()}
	w.Walk(rootScope, c.check)
	return c.diags
}

type w11eConjugatedChecker struct {
	resolver *resolve.Resolver
	diags    []diag.Diagnostic
}

func (c *w11eConjugatedChecker) check(sym *symbols.Symbol) {
	if rel, ok := sym.Decl.(*ast.RelationshipMember); ok {
		c.checkRelationshipMember(sym, rel)
		return
	}
	if !w11eHasConjugation(sym) {
		return
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || !w11eIsSpecialization(rel.Kind) || rel.Conjugated || rel.Target == nil {
			continue
		}
		c.report(w11eNameSpan(sym), msgConjugatedSpecific)
	}
}

// w11eIsSpecialization reports whether k is a Specialization in the KerML sense:
// subclassification, subsetting, redefinition and feature typing all are.
func w11eIsSpecialization(k ast.RelationshipKind) bool {
	switch k {
	case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines, ast.RelTyping:
		return true
	}
	return false
}

// checkRelationshipMember checks `subtype A specializes B` and the subset,
// redefinition and typing members, whose specific type is named rather than
// owning the relationship.
func (c *w11eConjugatedChecker) checkRelationshipMember(sym *symbols.Symbol, rel *ast.RelationshipMember) {
	if !w11eIsSpecialization(rel.Kind) || rel.Conjugated || rel.Source == nil || rel.Target == nil {
		return
	}
	specific, ok := c.resolver.ResolveTarget(kit.DeclarationScope(sym), rel.Source)
	if !ok || specific == nil {
		return
	}
	if w11eHasConjugation(specific) {
		c.report(rel.Source.Span(), msgConjugatedSpecific)
	}
}

// w11eHasConjugation reports whether sym declares itself the conjugate of
// another type, which makes it unspecializable. A conjugation is recorded as a
// conjugated generalization edge.
func w11eHasConjugation(sym *symbols.Symbol) bool {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel != nil && rel.Conjugated && rel.Kind == ast.RelSpecializes {
			return true
		}
	}
	return false
}

// w11eNameSpan returns the span of the declared name of sym, or of its whole
// declaration when it is unnamed.
func w11eNameSpan(sym *symbols.Symbol) source.Span {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		if d.Ident.Name != "" {
			return d.Ident.NameSpan
		}
	case *ast.Definition:
		if d.Ident.Name != "" {
			return d.Ident.NameSpan
		}
	}
	return sym.Decl.Span()
}

func (c *w11eConjugatedChecker) report(span source.Span, msg string) {
	for _, have := range c.diags {
		if have.Span.Offset == span.Offset && have.Message == msg {
			return
		}
	}
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     span,
		Message:  msg,
		Code:     "specialization-specific-conjugated",
		Source:   "type",
	})
}
