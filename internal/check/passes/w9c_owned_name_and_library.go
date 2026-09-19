package passes

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// W9CShortNameDistinguishabilityPass reports the short names one namespace uses
// twice, or that repeat another member's name: a short name is a name for
// distinguishability (KerML 7.2.2). Resolve's own rule keys on primary names,
// so it never sees these.
type W9CShortNameDistinguishabilityPass struct{}

func (W9CShortNameDistinguishabilityPass) Level() PassLevel { return LevelNameResolution }

func (W9CShortNameDistinguishabilityPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	byScope := map[*symbols.Scope][]*symbols.Symbol{}
	var order []*symbols.Scope
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		scope := sym.OwnerScope
		if scope == nil || sym.Decl == nil {
			return
		}
		if _, seen := byScope[scope]; !seen {
			order = append(order, scope)
		}
		byScope[scope] = append(byScope[scope], sym)
	})
	var diags []diag.Diagnostic
	for _, scope := range order {
		diags = append(diags, w9cShortNameConflicts(byScope[scope])...)
	}
	return diags
}

// w9cShortNameConflicts reports one diagnostic per member whose short name is
// another member's name or short name, at the repeated identifier.
func w9cShortNameConflicts(members []*symbols.Symbol) []diag.Diagnostic {
	uses := map[string][]*symbols.Symbol{}
	for _, sym := range members {
		for _, key := range w9cKeysOf(sym) {
			uses[key.name] = append(uses[key.name], sym)
		}
	}
	var diags []diag.Diagnostic
	for _, sym := range members {
		keys := w9cKeysOf(sym)
		for _, key := range keys {
			if len(uses[key.name]) < 2 || !w9cAnyShort(uses[key.name], key.name) {
				continue
			}
			diags = append(diags, diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Span:     key.span,
				Message:  "Duplicate of other owned member name",
				Code:     "name-conflict",
				Source:   "name-resolution",
			})
			break
		}
	}
	sort.SliceStable(diags, func(i, j int) bool { return diags[i].Span.Offset < diags[j].Span.Offset })
	return diags
}

// w9cAnyShort reports whether any member spells name as its short name, which
// is what makes the collision resolve's rule does not already report.
func w9cAnyShort(members []*symbols.Symbol, name string) bool {
	for _, sym := range members {
		if id, ok := w9cIdentOf(sym); ok && id.ShortName == name {
			return true
		}
	}
	return false
}

// w9cKey is one identifier a member is registered under, with its span.
type w9cKey struct {
	name string
	span source.Span
}

func w9cKeysOf(sym *symbols.Symbol) []w9cKey {
	id, ok := w9cIdentOf(sym)
	if !ok {
		return nil
	}
	var out []w9cKey
	if id.ShortName != "" {
		out = append(out, w9cKey{name: id.ShortName, span: id.ShortNameSpan})
	}
	if id.Name != "" && id.Name != id.ShortName {
		out = append(out, w9cKey{name: id.Name, span: id.NameSpan})
	}
	return out
}

func w9cIdentOf(sym *symbols.Symbol) (ast.Identification, bool) {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Ident, true
	case *ast.Usage:
		return d.Ident, true
	case *ast.Package:
		return d.Ident, true
	case *ast.Namespace:
		return d.Ident, true
	case *ast.SubjectMember:
		return d.Ident, true
	case *ast.AssumeMember:
		return d.Ident, true
	case *ast.RequireMember:
		return d.Ident, true
	}
	return ast.Identification{}, false
}

// w9cIsLibraryDocument reports whether a document is part of the standard
// library, whose packages are the ones entitled to be standard.
func w9cIsLibraryDocument(ctx *Context, name string) bool {
	scope := ctx.Index.DocumentRoot(name)
	if scope == nil {
		return false
	}
	found := false
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if sym != nil && ctx.Index.Library(sym) {
			found = true
			return false
		}
		return true
	})
	return found
}

// W9CUserStandardLibraryPass reports a `standard library package` written
// outside the standard library: only the library's own packages may claim it
// (KerML 7.2.4).
type W9CUserStandardLibraryPass struct{}

func (W9CUserStandardLibraryPass) Level() PassLevel { return LevelNameResolution }

func (W9CUserStandardLibraryPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	if w9cIsLibraryDocument(ctx, name) {
		return nil
	}
	var diags []diag.Diagnostic
	var walk func(nodes []ast.Node)
	walk = func(nodes []ast.Node) {
		for _, node := range nodes {
			switch n := node.(type) {
			case *ast.Membership:
				walk([]ast.Node{n.Member})
			case *ast.Package:
				if n.IsStandard {
					diags = append(diags, diag.Diagnostic{
						Severity: diag.SeverityWarning,
						Span:     source.Span{Offset: n.Span().Offset, Len: len("standard")},
						Message:  "User library packages should not be marked as standard",
						Code:     "library-package",
						Source:   "name-resolution",
					})
				}
				walk(n.Members)
			case *ast.Namespace:
				walk(n.Members)
			}
		}
	}
	walk(root.Members)
	return diags
}
