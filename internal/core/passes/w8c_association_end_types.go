package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// KerMLValidator's validateAssociationEndTypes message.
const msgAssociationEndTypes = "An association end must have exactly one type"

// AssociationEndTypesPass checks that every owned end feature of an association
// has exactly one type (KerML 8.3.4.1.2, validateAssociationEndTypes), counting
// types inherited by redefinition and ignoring ones a more specific type implies.
type AssociationEndTypesPass struct{}

func (AssociationEndTypesPass) Level() PassLevel { return LevelConstraint }

func (AssociationEndTypesPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &associationEndTypesChecker{resolver: ctx.Resolver(), model: ctx.Model()}
	w := &kit.Walker{Ctx: ctx}
	w.Walk(rootScope, c.check)
	return c.diags
}

type associationEndTypesChecker struct {
	resolver *resolve.Resolver
	model    *semantics.Model
	diags    []diag.Diagnostic
}

func (c *associationEndTypesChecker) check(assoc *symbols.Symbol) {
	if !w8cIsAssociation(assoc) || assoc.Scope == nil {
		return
	}
	assoc.Scope.ForEachMember(func(end *symbols.Symbol) bool {
		u, ok := w8cUsageOf(end)
		if !ok || !u.IsEnd {
			return true
		}
		// An end with no declared type still has exactly one — the implicit
		// Links::Link end type — so only two or more unrelated types conflict.
		if len(c.endTypes(assoc, end)) < 2 {
			return true
		}
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     u.Span(),
			Message:  msgAssociationEndTypes,
			Code:     "association-end-types",
			Source:   "constraint",
		})
		return true
	})
}

// endTypes returns the effective type set of an owned association end: its own
// typings plus those of the same-named ends it redefines in supertypes, with
// types subsumed by a more specific member removed.
func (c *associationEndTypesChecker) endTypes(assoc, end *symbols.Symbol) []*symbols.Symbol {
	types := c.declaredTypes(end)
	for _, super := range c.model.AllSupertypes(assoc) {
		if super == nil || super.Scope == nil || end.Name == "" {
			continue
		}
		super.Scope.ForEachMember(func(inherited *symbols.Symbol) bool {
			u, ok := w8cUsageOf(inherited)
			if !ok || !u.IsEnd || inherited.Name != end.Name {
				return true
			}
			types = append(types, c.declaredTypes(inherited)...)
			return true
		})
	}
	return w8cMostSpecific(c.model, types)
}

func (c *associationEndTypesChecker) declaredTypes(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		t, ok := c.resolver.ResolveTarget(kit.DeclarationScope(sym), rel.Target)
		if ok && t != nil {
			out = append(out, t)
		}
	}
	return out
}

// w8cMostSpecific dedupes types, dropping any that a different member conforms
// to (a supertype already implied by a more specific type).
func w8cMostSpecific(model *semantics.Model, types []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, t := range types {
		redundant := w8cContains(out, t)
		for _, o := range types {
			if o == t || redundant {
				continue
			}
			// A strictly more specific sibling subsumes t; mutually conforming
			// types are deduped by the first-occurrence check instead.
			if model.Conforms(o, t) && !model.Conforms(t, o) {
				redundant = true
			}
		}
		for _, o := range out {
			if model.Conforms(o, t) && model.Conforms(t, o) {
				redundant = true
			}
		}
		if redundant {
			continue
		}
		out = append(out, t)
	}
	return out
}

func w8cContains(syms []*symbols.Symbol, s *symbols.Symbol) bool {
	for _, x := range syms {
		if x == s {
			return true
		}
	}
	return false
}

// w8cIsAssociation reports whether sym declares an association (`assoc`,
// `assoc struct` or `connection def`-style association declaration).
func w8cIsAssociation(sym *symbols.Symbol) bool {
	u, ok := w8cUsageOf(sym)
	if !ok {
		return false
	}
	return u.Kind == ast.UsageAssoc
}

// w8cUsageOf returns sym's Usage declaration.
func w8cUsageOf(sym *symbols.Symbol) (*ast.Usage, bool) {
	if sym == nil {
		return nil, false
	}
	u, ok := sym.Decl.(*ast.Usage)
	return u, ok
}
