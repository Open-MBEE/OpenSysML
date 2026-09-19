package resolve

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Element filters (KerML 8.2.4, SysML v2 7.4.4) restrict which imported
// elements a namespace surfaces, never its declared ones. They are applied
// where a lookup enumerates the candidates an import surfaced, since only the
// semantic model can judge a condition (docs/project/spec-compliance.md).

// importAdmitsInto returns the test an element an import surfaces has to pass:
// the import's own filter clause (`import P::*[@Safety]`) and the `filter`
// members of the namespace declaring the import, which restrict every
// membership it imports — including a `filter` beside the `expose` lines of a
// view. When the import is inherited into the view owning into, what it exposes
// has to satisfy the conditions of that view and its supertypes as well.
func (r *Resolver) importAdmitsInto(into, scope *symbols.Scope, imp *ast.Import) func(*symbols.Symbol) bool {
	filters := r.namespaceFilters(scope)
	if imp.IsExpose {
		filters = appendConditions(filters, r.inheritedViewConditions(scope))
		if into != scope && isViewSymbol(into.Owner()) {
			filters = appendConditions(filters, r.namespaceFilters(into))
			filters = appendConditions(filters, r.inheritedViewConditions(into))
		}
	}
	if imp.FilterExpr != nil {
		filters = append(append([]symbols.ElementFilter{}, filters...), symbols.ElementFilter{
			Expr:  imp.FilterExpr,
			Scope: scope,
			Span:  imp.FilterExpr.Span(),
		})
	}
	if len(filters) == 0 {
		return func(*symbols.Symbol) bool { return true }
	}
	return func(sym *symbols.Symbol) bool { return r.admits(filters, sym) }
}

// appendConditions adds the conditions of more that filters do not state yet,
// copying filters before the first addition since callers memoize it.
func appendConditions(filters, more []symbols.ElementFilter) []symbols.ElementFilter {
	for _, f := range more {
		if !statesCondition(filters, f) {
			filters = append(append([]symbols.ElementFilter{}, filters...), f)
		}
	}
	return filters
}

// namespaceFilters are the conditions the namespace owning scope declares,
// extracted once per scope because every import lookup consults them.
func (r *Resolver) namespaceFilters(scope *symbols.Scope) []symbols.ElementFilter {
	if scope == nil {
		return nil
	}
	r.EnterDoc(symbols.DocNameOf(scope))
	defer r.LeaveDoc()
	if filters, ok := r.nsFilters[scope]; ok {
		return filters
	}
	filters := symbols.NamespaceFiltersIn(scope)
	// A namespace can be declared by more than one document, and a filter any of
	// its declarations states restricts every membership it imports — the same
	// conditions the index gates its re-exports with.
	if fqn := r.namespaceFQNOf(scope); fqn != "" {
		for _, f := range r.idx.NamespaceFiltersOf(fqn) {
			if !statesCondition(filters, f) {
				filters = append(filters, f)
			}
		}
	}
	journalNew(r, r.nsFilters, scope, scope.Node())
	r.nsFilters[scope] = filters
	return filters
}

// inheritedViewConditions are the `filter` members of the views the view owning
// scope specializes or is typed by: what a view exposes has to satisfy the
// conditions of its definition too (SysML v2 7.24.2).
func (r *Resolver) inheritedViewConditions(scope *symbols.Scope) []symbols.ElementFilter {
	if scope == nil || !isViewSymbol(scope.Owner()) {
		return nil
	}
	r.EnterDoc(symbols.DocNameOf(scope))
	defer r.LeaveDoc()
	if filters, ok := r.viewFilters[scope]; ok {
		return filters
	}
	supers, ok := r.model.(supertypeProvider)
	if !ok || r.viewFiltersInProgress[scope] {
		return nil
	}
	// Resolving the view's type looks through its own exposes, which asks again.
	r.viewFiltersInProgress[scope] = true
	defer delete(r.viewFiltersInProgress, scope)

	var filters []symbols.ElementFilter
	seen := map[*symbols.Symbol]bool{scope.Owner(): true}
	queue := supers.DirectSupertypes(scope.Owner())
	for len(queue) > 0 {
		super := queue[0]
		queue = queue[1:]
		if super == nil || seen[super] || !isViewSymbol(super) {
			continue
		}
		seen[super] = true
		for _, f := range r.viewOwnConditions(super) {
			if !statesCondition(filters, f) {
				filters = append(filters, f)
			}
		}
		queue = append(queue, supers.DirectSupertypes(super)...)
	}
	journalNew(r, r.viewFilters, scope, scope.Node())
	r.viewFilters[scope] = filters
	return filters
}

// viewOwnConditions are the `filter` members view declares: read from its scope
// when it has one, else from the index a restored library populates.
func (r *Resolver) viewOwnConditions(view *symbols.Symbol) []symbols.ElementFilter {
	if view.Scope != nil {
		return r.namespaceFilters(view.Scope)
	}
	if r.idx == nil {
		return nil
	}
	if fqn := r.idx.GetFQN(view); fqn != "" {
		return r.idx.NamespaceFiltersOf(fqn)
	}
	return nil
}

// isViewSymbol reports whether sym declares a view definition or usage.
func isViewSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolViewUsage, symbols.SymbolViewDef:
		return true
	}
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		return decl.Kind == ast.DefView
	case *ast.Usage:
		return decl.Kind == ast.UsageView
	}
	return false
}

// namespaceFQNOf is the indexed name of the namespace owning scope, or "" for a
// root or unnamed one, whose filters no other declaration shares.
func (r *Resolver) namespaceFQNOf(scope *symbols.Scope) string {
	if r.idx == nil || scope.Owner() == nil || scope.Owner().Name == "" {
		return ""
	}
	return r.idx.GetFQN(scope.Owner())
}

// statesCondition reports whether filters already hold f's condition, which the
// namespace's own declaration and the index both answer for.
func statesCondition(filters []symbols.ElementFilter, f symbols.ElementFilter) bool {
	return slices.ContainsFunc(filters, func(have symbols.ElementFilter) bool {
		return have.Same(f) || (have.Expr != nil && have.Expr == f.Expr)
	})
}

// documentOf names the document scope belongs to, which decides the routes to a
// root-level name: each document's root namespace is its own.
func (r *Resolver) documentOf(scope *symbols.Scope) string {
	if r.idx == nil {
		return ""
	}
	if doc := r.idx.DocumentOfRoot(rootOf(scope)); doc != "" {
		return doc
	}
	// A document builds its own scope tree for the editor, which the index does
	// not hold, so identify it by the document name stamped on its symbols.
	return symbols.DocNameOf(scope)
}

// admitsUnderName reports whether cand is reachable under the name fqn from the
// namespace from: a route re-exporting it has to reach a lookup made there
// (symbols.Index.ReexportVisible) and one route's conditions all have to hold
// (symbols.Index.ReexportGates).
func (r *Resolver) admitsUnderName(doc, from, fqn string, cand *symbols.Symbol) bool {
	if r.idx == nil {
		return true
	}
	if !r.idx.ReexportVisible(doc, fqn, cand) {
		return false
	}
	routes := r.idx.ReexportGates(doc, fqn, cand, from)
	if len(routes) == 0 {
		return true
	}
	for _, route := range routes {
		if r.admits(route, cand) {
			return true
		}
	}
	return false
}

// admittedUnder keeps the candidates a lookup of the name fqn may reach, i.e.
// the ones the conditions gating that name select (see admitsUnderName).
func (r *Resolver) admittedUnder(doc, from, fqn string, cands []*symbols.Symbol) []*symbols.Symbol {
	kept := cands
	for i, sym := range cands {
		if r.admitsUnderName(doc, from, fqn, sym) {
			continue
		}
		// Copy on first rejection, so an unfiltered lookup keeps its slice.
		kept = make([]*symbols.Symbol, 0, len(cands))
		kept = append(kept, cands[:i]...)
		for _, s := range cands[i+1:] {
			if r.admitsUnderName(doc, from, fqn, s) {
				kept = append(kept, s)
			}
		}
		return kept
	}
	return kept
}

// AdmittedChildrenOf keeps the members of the namespace fqn that a lookup made
// in scope can reach, so an enumeration of a namespace (completion) offers only
// names resolution then admits.
func (r *Resolver) AdmittedChildrenOf(scope *symbols.Scope, fqn string, children []*symbols.Symbol) []*symbols.Symbol {
	if fqn == "" || len(children) == 0 {
		return children
	}
	doc, from := r.documentOf(scope), r.ReferringNamespaceFQN(scope)
	kept := make([]*symbols.Symbol, 0, len(children))
	for _, sym := range children {
		// A qualified name reaches only the namespace's visible memberships.
		if !r.namedThroughNamespace(sym) {
			continue
		}
		if r.admitsUnderName(doc, from, fqn+"::"+localNameOf(sym), sym) {
			kept = append(kept, sym)
		}
	}
	return kept
}

// AdmittedTopLevel keeps the root-level names a lookup made in doc reaches, so
// an enumeration of them (completion) offers only names resolution then admits:
// a name another document's import borrowed, or one this document's import
// filter rejects, is no member of this document's root namespace.
func (r *Resolver) AdmittedTopLevel(doc string, bindings []symbols.RootBinding) []*symbols.Symbol {
	kept := make([]*symbols.Symbol, 0, len(bindings))
	for _, b := range bindings {
		if r.admitsUnderName(doc, "", b.Name, b.Sym) {
			kept = append(kept, b.Sym)
		}
	}
	return kept
}

// ImportedElements enumerates the elements imp surfaces into scope, with the
// same admission a lookup through imp makes: the import's filter clause and the
// `filter` members of the declaring namespace (see importAdmitsInto).
func (r *Resolver) ImportedElements(scope *symbols.Scope, imp *ast.Import) []*symbols.Symbol {
	return r.ImportedElementsInto(scope, scope, imp)
}

// ImportedElementsInto enumerates the elements imp, declared in scope, surfaces
// into the namespace owning into, which inherits it: an inherited `expose` is
// admitted against the inheriting view's conditions too (see importAdmitsInto).
func (r *Resolver) ImportedElementsInto(into, scope *symbols.Scope, imp *ast.Import) []*symbols.Symbol {
	if scope == nil || imp == nil || imp.Imported == nil || len(imp.Imported.Parts) == 0 {
		return nil
	}
	if into == nil {
		into = scope
	}
	target, ok := r.resolveImportTarget(scope, imp)
	if !ok || target == nil {
		return nil
	}
	admit := r.importAdmitsInto(into, scope, imp)
	out := newElementList()
	if imp.Kind == ast.ImportMembership {
		// `import P::x` surfaces x itself; the recursive form adds its subtree.
		// An import of an alias imports that membership, so the name it surfaces
		// is the alias's own (KerML §8.2.3.2).
		named := target
		if alias, ok := r.PartAlias(imp.Imported, len(imp.Imported.Parts)-1); ok {
			named = alias
		}
		if admit(target) {
			out.add(named)
		}
	} else {
		r.appendNamespaceMembers(out, scope, target, imp, admit)
	}
	if imp.IsRecursive {
		r.appendSubtree(out, scope, target, imp, admit, map[symbols.ElementKey]bool{})
	}
	return out.elems
}

// appendNamespaceMembers adds the members of target a namespace import surfaces.
func (r *Resolver) appendNamespaceMembers(out *elementList, scope *symbols.Scope, target *symbols.Symbol, imp *ast.Import, admit func(*symbols.Symbol) bool) {
	for _, sym := range r.namespaceChildren(scope, target, imp) {
		if visibleThroughImport(imp, sym) && admit(sym) {
			out.add(sym)
		}
	}
}

// namespaceChildren lists the members of target an import reaches, the declared
// ones first and then the ones only the index holds — wildcard imports and
// restored libraries populate the index rather than a scope. Reachability under
// the name is decided here; visibility and element filters are not.
func (r *Resolver) namespaceChildren(scope *symbols.Scope, target *symbols.Symbol, imp *ast.Import) []*symbols.Symbol {
	children := newElementList()
	if target.Scope != nil {
		for _, sym := range target.Scope.Members() {
			children.add(sym)
		}
		for _, childImp := range r.importsOf(target.Scope.Node()) {
			if !r.importVisibleFrom(target, scope, childImp) || r.importStack[childImp] {
				continue
			}
			r.importStack[childImp] = true
			for _, sym := range r.ImportedElements(target.Scope, childImp) {
				children.add(sym)
			}
			delete(r.importStack, childImp)
		}
	}
	if r.idx == nil {
		return children.elems
	}
	prefix := r.indexedNameOf(target)
	var indexed []*symbols.Symbol
	if importAllowsPrivate(imp) {
		indexed = r.idx.LookupDirectChildren(prefix)
	} else {
		indexed = r.idx.LookupDirectChildrenFrom(prefix, r.ReferringNamespaceFQN(scope))
	}
	for _, sym := range indexed {
		if r.admitsUnderName("", r.ReferringNamespaceFQN(scope), prefix+"::"+localNameOf(sym), sym) {
			children.add(sym)
		}
	}
	return children.elems
}

// indexedNameOf is the qualified name the index keys target's children under. A
// symbol declared in a document carries its local name, so the nesting has to
// come from the index.
func (r *Resolver) indexedNameOf(target *symbols.Symbol) string {
	if r.idx == nil {
		return target.Name
	}
	if fqn := r.idx.GetFQN(target); fqn != "" {
		return fqn
	}
	return target.Name
}

// appendSubtree adds the descendants of target a recursive import surfaces. The
// walk descends through the members the import can see, as the lookup does: a
// namespace it cannot surface hides its own contents too (see eachSubtreeMatch).
func (r *Resolver) appendSubtree(out *elementList, scope *symbols.Scope, target *symbols.Symbol, imp *ast.Import, admit func(*symbols.Symbol) bool, seen map[symbols.ElementKey]bool) {
	if target == nil || (target.Scope != nil && target.Scope.BodyLocal()) {
		return
	}
	key := symbols.KeyOf(target)
	if seen[key] {
		return
	}
	seen[key] = true
	children := r.namespaceChildren(scope, target, imp)
	for _, sym := range children {
		if visibleThroughImport(imp, sym) && admit(sym) {
			out.add(sym)
		}
	}
	for _, sym := range children {
		if !visibleThroughImport(imp, sym) {
			continue
		}
		r.appendSubtree(out, scope, sym, imp, admit, seen)
	}
}

// elementList collects elements in the order they were surfaced, once each,
// identified by declaration: a member reached through both its scope and the
// index is one element, and the scope's symbol is kept since it is reached first.
type elementList struct {
	elems []*symbols.Symbol
	seen  map[symbols.ElementKey]bool
}

func newElementList() *elementList {
	return &elementList{seen: map[symbols.ElementKey]bool{}}
}

func (l *elementList) add(sym *symbols.Symbol) {
	if sym == nil {
		return
	}
	key := symbols.KeyOf(sym)
	if l.seen[key] {
		return
	}
	l.seen[key] = true
	l.elems = append(l.elems, sym)
}

// localNameOf is the last segment of sym's name, i.e. the name it is a member under.
func localNameOf(sym *symbols.Symbol) string {
	if i := strings.LastIndex(sym.Name, "::"); i >= 0 {
		return sym.Name[i+len("::"):]
	}
	return sym.Name
}

// admits reports whether cand satisfies every one of filters. A condition the
// model cannot judge keeps the candidate, so an unevaluable filter never
// silently hides model content; the filter pass reports it.
func (r *Resolver) admits(filters []symbols.ElementFilter, cand *symbols.Symbol) bool {
	if len(filters) == 0 || cand == nil {
		return true
	}
	// A condition's own names are resolved unfiltered, or the condition would
	// filter itself: judging a candidate compiles the condition, which cannot
	// resolve a name whose resolution is what asked.
	if r.inCondition > 0 {
		return true
	}
	judge, ok := r.model.(elementFilterChecker)
	if !ok {
		return true
	}
	admitted := true
	// Evaluating a condition reaches declarations of other documents, which this
	// document does not report on.
	r.aside(func() {
		for _, f := range filters {
			if f.IsZero() {
				continue
			}
			if !judge.SatisfiesElementFilter(f, cand) {
				admitted = false
				return
			}
		}
	})
	return admitted
}
