package view

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// exposeSite is one expose relationship of a view or of a view it specializes.
type exposeSite struct {
	owner *symbols.Symbol
	imp   *ast.Import
}

// exposureReason says why a rendering of view shows nothing, in the terms of
// the view's exposure: it exposes nothing, names something that does not
// resolve, filters out everything it exposes (a matrix exposes unnamed members
// too), or exposes elements this kind of rendering does not show. "" leaves the
// rendering's own account to stand.
func (r *Renderer) exposureReason(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) string {
	if len(exposed) > 0 {
		if len(out.Notices) > 0 {
			return ""
		}
		return fmt.Sprintf("none of the %d element(s) the view exposes is shown by %s %s rendering; the rendering is empty",
			len(exposed), out.Kind.article(), out.Kind)
	}
	var exposes []exposeSite
	for _, sym := range append([]*symbols.Symbol{view}, r.model.AllSupertypes(view)...) {
		if !semantics.IsView(sym) || sym.Scope == nil {
			continue
		}
		for _, imp := range sym.Scope.Imports() {
			if imp.IsExpose {
				exposes = append(exposes, exposeSite{owner: sym, imp: imp})
			}
		}
	}
	if len(exposes) == 0 {
		return "the view exposes nothing; the rendering is empty"
	}
	candidates := 0
	if r.resolver != nil {
		for _, site := range exposes {
			if _, ok := r.resolver.ImportTarget(site.owner.Scope, site.imp); !ok {
				return fmt.Sprintf("the view exposes %s, which does not resolve; the rendering is empty", exposePhrase(site.imp))
			}
			candidates += len(r.resolver.ImportedMembersUnfiltered(site.owner.Scope, site.imp, out.Kind == KindMatrix))
		}
	}
	// A filter is the reason only when it had elements to reject.
	if filters := r.viewFilters(view); len(filters) > 0 && (candidates > 0 || r.resolver == nil) {
		return fmt.Sprintf("every element the view exposes is rejected by its filter %s; the rendering is empty", r.filtersText(filters))
	}
	spelled := make([]string, len(exposes))
	for i, site := range exposes {
		spelled[i] = exposePhrase(site.imp)
	}
	return fmt.Sprintf("the view exposes %s, and there is nothing there; the rendering is empty", strings.Join(spelled, " and "))
}

// exposedReason says why a rendering of elements exposed without a view — a
// document rendered with none declared — shows nothing.
func exposedReason(exposed []*symbols.Symbol, out *Rendering) string {
	if len(exposed) == 0 {
		return "the document declares no element; the rendering is empty"
	}
	if len(out.Notices) > 0 {
		return ""
	}
	return fmt.Sprintf("none of the %d element(s) the document declares is shown by %s %s rendering; the rendering is empty",
		len(exposed), out.Kind.article(), out.Kind)
}

// exposePhrase says what an expose reaches, in words rather than in the
// notation's `P::*`, so the reason reads as prose wherever the text form goes.
func exposePhrase(imp *ast.Import) string {
	name := imp.Imported.Text()
	switch {
	case imp.IsRecursive:
		return "everything under " + name
	case imp.Kind != ast.ImportMembership:
		return "the members of " + name
	default:
		return name
	}
}

// filtersText spells element filters as they were written, each in backticks,
// or counts them when the rendering holds no source for them.
func (r *Renderer) filtersText(filters []symbols.ElementFilter) string {
	var spelled []string
	for _, f := range filters {
		var text string
		if r.text != nil && f.Scope != nil && f.Expr != nil && f.Expr.Span().Len > 0 {
			text = collapseSpace(r.text(f.Scope.DocName(), f.Expr.Span()))
		}
		if text == "" {
			if len(filters) == 1 {
				return "condition"
			}
			return fmt.Sprintf("%d conditions", len(filters))
		}
		spelled = append(spelled, text)
	}
	return strings.Join(spelled, " and ")
}

// blankNotice is what a consumer of the rendering's nodes, edges and rows is
// told when it gets none of them: why the rendering is empty, or that it is
// pictures alone, which such a consumer does not draw.
func (r *Rendering) blankNotice() string {
	if r.Empty() {
		return r.EmptyReason()
	}
	return fmt.Sprintf("the view shows %d picture(s) and no element", len(r.Pictures))
}
