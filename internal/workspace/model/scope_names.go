package model

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// VisibleName is one spelling resolution would consider at a point: the dotted
// path a reference there may be written as, and the element it reaches.
type VisibleName struct {
	// Name is the path in the pilot's notation, segments joined by ".".
	Name string
	// FQN is the "::"-qualified name of the element the path reaches.
	FQN string
	// Kind classifies the element, an alias resolved to its target: a caller
	// restricting the set by metatype — a specialization names a type, a
	// redefinition names a feature — needs what the name reaches, not the name.
	Kind symbols.SymbolKind
	// Depth is the number of segments in Name.
	Depth int
}

// VisibleNamesOptions bounds a visible-name enumeration.
type VisibleNamesOptions struct {
	// Redefinition enumerates only what the anchor's owner inherits: a
	// redefinition's target is resolved against the generals of the owning type
	// rather than against its own members (KerML 8.3.3.3.6).
	Redefinition bool
	// LibraryRoots names the standard-library root packages the caller counts
	// as loaded: `Base` alone admits the implicit `self` and `that` it
	// contributes to a declaration. A library element is reachable through
	// inheritance and imports only when its root is named here, and a library
	// root package is never offered as a top-level name of its own.
	LibraryRoots []string
	// MaxDepth bounds the number of segments in an enumerated path.
	MaxDepth int
}

// defaultVisibleNameDepth bounds path length. What terminates the enumeration
// is the one-re-entry rule, so this is only a safety valve.
const defaultVisibleNameDepth = 9

// maxNameOccurrences is the one-re-entry rule: a path may name an element twice
// but not a third time, so a containment cycle is observable exactly once.
const maxNameOccurrences = 2

// VisibleNames enumerates every name resolution would consider in scope: the
// members of the enclosing namespace chain, what those namespaces inherit and
// import, and the document roots the index holds, each under every path it can
// be reached by. It is the read-only enumeration behind the single-name lookup
// resolve performs, and it answers both halves of a visibility question — a
// name that should be reachable and is not, and one that is reachable and
// should not be. Results are sorted by name, so two runs agree byte for byte.
func (w *Workspace) VisibleNames(scope *symbols.Scope, opts VisibleNamesOptions) []VisibleName {
	if scope == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	doc := symbols.DocNameOf(scope)
	var out []VisibleName
	w.queryLocked(doc, func(r *resolve.Resolver, sem *semantics.Model) {
		nw := &nameWalk{
			idx:      w.index,
			r:        r,
			sem:      sem,
			doc:      doc,
			maxDepth: opts.MaxDepth,
			library:  map[string]bool{},
			seen:     map[string]bool{},
			at:       map[string]*symbols.Symbol{},
		}
		for _, root := range opts.LibraryRoots {
			nw.library[root] = true
		}
		if nw.maxDepth <= 0 {
			nw.maxDepth = defaultVisibleNameDepth
		}
		nw.walk(scope, opts.Redefinition)
		out = nw.out
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// VisibleNamesAt is VisibleNames for the deepest scope of a document that
// contains offset.
func (w *Workspace) VisibleNamesAt(doc string, offset int, opts VisibleNamesOptions) []VisibleName {
	d := w.Document(doc)
	if d == nil || d.Scope == nil {
		return nil
	}
	return w.VisibleNames(scopeAt(d.Scope, offset), opts)
}

// ScopeAt is the deepest scope of a document whose declaration contains offset,
// which is the namespace a name written there is resolved in.
func (w *Workspace) ScopeAt(doc string, offset int) *symbols.Scope {
	d := w.Document(doc)
	if d == nil || d.Scope == nil {
		return nil
	}
	return scopeAt(d.Scope, offset)
}

// scopeAt returns the deepest scope whose owning declaration contains offset.
func scopeAt(root *symbols.Scope, offset int) *symbols.Scope {
	for _, sym := range root.Members() {
		if sym.Scope == nil {
			continue
		}
		if sp := sym.DeclSpan; offset >= sp.Offset && offset < sp.End() {
			return scopeAt(sym.Scope, offset)
		}
	}
	return root
}

// nameWalk enumerates the paths reachable from one anchor scope. Each path is
// recorded once, the first time it is reached, which truncates the circularity
// an import or a specialization cycle would otherwise spin in.
type nameWalk struct {
	idx      *symbols.Index
	r        *resolve.Resolver
	sem      *semantics.Model
	doc      string
	maxDepth int
	// library are the standard-library root packages counted as loaded.
	library map[string]bool
	seen    map[string]bool
	// at is the element each recorded path was recorded for.
	at map[string]*symbols.Symbol
	// reentry is set while walking a same-named element under a path already
	// recorded for another one: the path is not offered twice, but what it
	// reaches through this element is.
	reentry int
	out     []VisibleName
	// expanding counts how often the current path is enumerating an element's
	// members, so a containment cycle is re-entered once and no more.
	expanding map[*symbols.Symbol]int
	// traversed counts the types the current path's derivation went through, so
	// a path never inherits through the same type twice.
	traversed map[*symbols.Symbol]int
	// noImplicit is set while the path may not continue through an implicit
	// general, which a recursive import suppresses.
	noImplicit int
	// redefAnchor is the namespace a redefinition is written in, whose own
	// declarations it cannot name.
	redefAnchor *symbols.Symbol
	// chains memoizes the source steps from an owner to a member's declarer.
	chains map[[2]*symbols.Symbol][]*symbols.Symbol
	// pending maps a type to the declaration being written in it, whose
	// redefinition masks nothing there (KerML 8.3.3.3.6).
	pending map[*symbols.Symbol]pendingDecl
}

// pendingDecl is the declaration being written in a type. sym is nil when the
// walk cannot tell which of the type's declarations it is; hide reports whether
// that declaration is not yet a member of the type.
type pendingDecl struct {
	sym  *symbols.Symbol
	hide bool
}

// markPending records the declaration being written in a type. A package
// declares no features, so nothing is pending in it.
func (nw *nameWalk) markPending(owner *symbols.Symbol, decl pendingDecl) {
	if owner == nil || owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace {
		return
	}
	nw.pending[owner] = decl
}

// membersOf reports the members of owner as the walk sees them: the declaration
// being written in it is absent, and the redefinitions it declares mask nothing.
func (nw *nameWalk) membersOf(owner *symbols.Symbol) []*symbols.Symbol {
	if decl, ok := nw.pending[owner]; ok {
		return nw.sem.MembersOfDeclaring(owner, decl.sym)
	}
	return nw.sem.MembersOf(owner)
}

// walk enumerates the anchor scope and its ancestors, then the index roots.
func (nw *nameWalk) walk(anchor *symbols.Scope, redefinition bool) {
	nw.expanding = map[*symbols.Symbol]int{}
	nw.pending = map[*symbols.Symbol]pendingDecl{}
	nw.traversed = map[*symbols.Symbol]int{}
	nw.chains = map[[2]*symbols.Symbol][]*symbols.Symbol{}
	if redefinition && anchor != nil {
		// The anchor is the namespace the redefinition is written in: one of
		// its declarations is the redefining one, and which is not knowable
		// from a scope alone.
		owner := anchor.Owner()
		nw.markPending(owner, pendingDecl{hide: true})
		// It may instead be the redefining declaration's own scope — a cursor
		// in its head — in which case the namespace is its owner, and there the
		// declaration is known and stays visible under its own name.
		if semantics.DeclaresRedefinition(owner) && owner.OwnerScope != nil {
			nw.markPending(owner.OwnerScope.Owner(), pendingDecl{sym: owner})
		}
	}
	for s := anchor; s != nil; s = s.Parent() {
		if s == anchor && redefinition {
			// A redefinition names a feature its owner inherits, never one the
			// owner declares itself.
			nw.redefAnchor = s.Owner()
			nw.inherited(s.Owner(), "", 1, true)
			nw.redefAnchor = nil
			continue
		}
		nw.locals(s, "", 1, true, true)
		nw.inherited(s.Owner(), "", 1, true)
		nw.imports(s, "", 1, true, false)
	}
	nw.roots()
}

// roots adds the names the index registers at its root: every non-library
// document's top-level declarations.
func (nw *nameWalk) roots() {
	rooted := map[string]*symbols.Symbol{}
	for _, b := range nw.idx.TopLevelBindings(nw.doc) {
		if nw.idx.Library(b.Sym) {
			continue
		}
		if prev, ok := rooted[b.Name]; ok {
			// Two documents declare the same top-level name and neither shadows
			// the other, so both subtrees are reachable under it even though the
			// name itself resolves to the first.
			if t := nw.targetOf(b.Sym); t != prev && nw.loaded(t) {
				nw.reentry++
				nw.expand(b.Name, t, 2)
				nw.reentry--
			}
			continue
		}
		before := len(nw.out)
		nw.add("", b.Name, b.Sym, 1)
		if len(nw.out) > before {
			rooted[b.Name] = nw.at[b.Name]
		}
	}
}

// targetOf is the element a name reaches, an alias replaced by its target.
func (nw *nameWalk) targetOf(sym *symbols.Symbol) *symbols.Symbol {
	if t, ok := nw.r.ResolveAliasTarget(sym); ok && t != nil {
		return t
	}
	return sym
}

// locals adds what a scope declares directly. inside admits its private
// members, as a reference written within the namespace sees them; inheriting
// admits protected ones, which a specialization sees (KerML 8.2.3.3).
func (nw *nameWalk) locals(s *symbols.Scope, prefix string, depth int, inside, inheriting bool) {
	if s == nil {
		return
	}
	decl := nw.pending[s.Owner()]
	// The binding name is the key, not the symbol's own name: an alias member
	// binds a name to another element (KerML 7.3.4.4).
	for _, name := range s.MemberNames() {
		for _, sym := range s.LookupLocalAll(name) {
			if !symbols.VisibleAs(sym.Visibility, inside, inheriting) {
				continue
			}
			if decl.hide && semantics.NotYetMember(sym, decl.sym) {
				continue // not yet a member of the namespace it is written in
			}
			nw.add(prefix, leafName(name), sym, depth)
			nw.add(prefix, sym.ShortName, sym, depth)
		}
	}
}

// inherited adds the members owner takes from what it specializes, is typed by
// or reference-subsets, which the semantic model reports with masking applied.
func (nw *nameWalk) inherited(owner *symbols.Symbol, prefix string, depth int, inheriting bool) {
	// Only a Type has generals to inherit from: a package or a namespace has
	// no implicit specialization, so it contributes nothing here.
	if owner == nil || owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace {
		return
	}
	// One filter chain: the semantic member view drops what a redefinition
	// masks, then membership visibility drops what cannot be named here, then
	// the path's own derivation drops what it would inherit twice over.
	var declared map[*symbols.Symbol]bool
	if nw.noImplicit > 0 {
		declared = nw.declaredSources(owner)
	}
	for _, sym := range nw.membersOf(owner) {
		if !symbols.VisibleAs(sym.Visibility, false, inheriting) {
			continue
		}
		if declared != nil && !declared[declarerOf(sym)] {
			continue
		}
		if owner == nw.redefAnchor && declarerOf(sym) == owner {
			continue
		}
		chain, ok := nw.enter(nw.chainTo(owner, declarerOf(sym)))
		if !ok {
			// The shortest chain is spent; the member is still inherited if
			// another chain of sources reaches its declarer.
			if detour, found := nw.chainAvoiding(owner, declarerOf(sym)); found {
				chain, ok = nw.enter(detour)
			}
		}
		if !ok {
			continue
		}
		nw.add(prefix, leafName(sym.Name), sym, depth)
		nw.add(prefix, sym.ShortName, sym, depth)
		nw.leave(chain)
	}
	nw.inheritedImports(owner, prefix, depth, map[*symbols.Symbol]bool{})
}

// declaredSources returns owner and what it reaches through declared
// specializations, leaving out the implicit generals.
func (nw *nameWalk) declaredSources(owner *symbols.Symbol) map[*symbols.Symbol]bool {
	out := map[*symbols.Symbol]bool{owner: true}
	queue := []*symbols.Symbol{owner}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		implicit := map[*symbols.Symbol]bool{}
		for _, imp := range nw.sem.ImplicitGenerals(cur) {
			implicit[imp] = true
		}
		next := append([]*symbols.Symbol{}, nw.sem.DirectSupertypes(cur)...)
		if ref := nw.sem.ReferencedFeature(cur); ref != nil {
			next = append(next, ref)
		}
		for _, src := range next {
			if src == nil || out[src] || implicit[src] {
				continue
			}
			out[src] = true
			queue = append(queue, src)
		}
	}
	return out
}

func (nw *nameWalk) inheritedImports(owner *symbols.Symbol, prefix string, depth int, seen map[*symbols.Symbol]bool) {
	for _, src := range nw.sem.DirectMemberSources(owner) {
		if src == nil || seen[src] {
			continue
		}
		seen[src] = true
		chain, ok := nw.enterFor(depth, nw.chainTo(owner, src))
		if !ok {
			continue
		}
		if src.Scope != nil {
			nw.imports(src.Scope, prefix, depth, false, true)
		}
		nw.inheritedImports(src, prefix, depth, seen)
		nw.leave(chain)
	}
}

// declarerOf is the element whose scope declares sym, which is the type a path
// inheriting sym went through.
func declarerOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// chainTo returns the source steps from owner to declarer over the
// member-contributing edges: the types a path reaching declarer's members
// through owner inherits through. A declared generalization is preferred to an
// implicit base — a member of a feature's type is inherited through that type,
// not through Base::things — and a shorter chain to a longer one. It is empty
// when owner declares them itself or when no chain of sources reaches declarer.
func (nw *nameWalk) chainTo(owner, declarer *symbols.Symbol) []*symbols.Symbol {
	if owner == nil || declarer == nil || declarer == owner {
		return nil
	}
	key := [2]*symbols.Symbol{owner, declarer}
	if chain, ok := nw.chains[key]; ok {
		return chain
	}
	from, found := nw.searchChain(owner, declarer, nil)
	var chain []*symbols.Symbol
	if found {
		for s := declarer; s != nil && s != owner; s = from[s] {
			chain = append(chain, s)
		}
	}
	nw.chains[key] = chain
	return chain
}

// searchChain walks the member-contributing edges out of owner towards
// declarer, taking every declared generalization before any implicit base, and
// skipping the sources skip reports. It reports the step each source was
// reached from, and whether declarer was reached at all.
func (nw *nameWalk) searchChain(
	owner, declarer *symbols.Symbol,
	skip func(from, src *symbols.Symbol) bool,
) (map[*symbols.Symbol]*symbols.Symbol, bool) {
	from := map[*symbols.Symbol]*symbols.Symbol{owner: nil}
	declared, implicit := []*symbols.Symbol{owner}, []*symbols.Symbol(nil)
	for len(declared) > 0 || len(implicit) > 0 {
		var cur *symbols.Symbol
		if len(declared) > 0 {
			cur, declared = declared[0], declared[1:]
		} else {
			cur, implicit = implicit[0], implicit[1:]
		}
		general := map[*symbols.Symbol]bool{}
		for _, g := range nw.sem.DirectSupertypes(cur) {
			general[g] = true
		}
		for _, src := range nw.sem.DirectMemberSources(cur) {
			if src == nil || (skip != nil && skip(cur, src)) {
				continue
			}
			if _, seen := from[src]; seen {
				continue
			}
			from[src] = cur
			if src == declarer {
				return from, true
			}
			if general[src] {
				declared = append(declared, src)
			} else {
				implicit = append(implicit, src)
			}
		}
	}
	return from, false
}

// enter marks a derivation's source steps as traversed on the current path, or
// reports false when one of them already is: the pilot's enumeration stops
// there rather than inheriting through a type twice (KerML 8.2.3.5).
func (nw *nameWalk) enter(chain []*symbols.Symbol) ([]*symbols.Symbol, bool) {
	for _, src := range chain {
		if nw.traversed[src] > 0 {
			return nil, false
		}
	}
	for _, src := range chain {
		nw.traversed[src]++
	}
	return chain, true
}

// enterFor marks an inherited import's steps once the path has left the
// anchor's own scope: how a name reaches the anchor is not a step of the
// paths under it.
func (nw *nameWalk) enterFor(depth int, chain []*symbols.Symbol) ([]*symbols.Symbol, bool) {
	if depth <= 1 {
		return nil, true
	}
	return nw.enter(chain)
}

// leave undoes enter.
func (nw *nameWalk) leave(chain []*symbols.Symbol) {
	for _, src := range chain {
		nw.traversed[src]--
	}
}

// imports adds the imports visible inside a namespace or through inheritance.
func (nw *nameWalk) imports(s *symbols.Scope, prefix string, depth int, inside, inheriting bool) {
	if s == nil {
		return
	}
	for _, imp := range importsIn(s.Node()) {
		if !symbols.VisibleAs(imp.Visibility, inside, inheriting) {
			continue
		}
		nw.importOne(s, imp, prefix, depth, inheriting)
	}
}

// importOne adds the names one import surfaces into a namespace. A wildcard
// import reaches into the imported namespace, so it counts as a derivation step
// through it: a path crosses each namespace's imports once (KerML 8.2.3.5).
func (nw *nameWalk) importOne(s *symbols.Scope, imp *ast.Import, prefix string, depth int, inheriting bool) {
	if imp.Kind != ast.ImportMembership {
		if target, ok := nw.r.ImportTarget(s, imp); ok && target != nil {
			chain, ok := nw.enter([]*symbols.Symbol{target})
			if !ok {
				return
			}
			defer nw.leave(chain)
		}
	}
	for i, sym := range nw.r.ImportedElements(s, imp) {
		// A membership import surfaces only the name written last in it,
		// which for an alias membership is the alias name, not the target's;
		// its recursive form adds the subtree under their own names.
		direct := imp.Kind == ast.ImportMembership && i == 0
		// A name found by a recursive import's descent carries no implicitly
		// inherited members; the membership named directly still does.
		if imp.IsRecursive && !direct {
			nw.noImplicit++
		}
		if direct {
			written := importedName(imp)
			nw.add(prefix, written, sym, depth)
			// An element imported by one of its own two names keeps both;
			// an alias name belongs to the alias, not to its target.
			if written == leafName(sym.Name) || written == sym.ShortName {
				nw.add(prefix, leafName(sym.Name), sym, depth)
				nw.add(prefix, sym.ShortName, sym, depth)
			}
		} else {
			nw.add(prefix, leafName(sym.Name), sym, depth)
			nw.add(prefix, sym.ShortName, sym, depth)
			// The descent resolves in each namespace it reaches, so what a
			// type there declares as a supertype's member is named there too.
			if imp.IsRecursive {
				nw.inherited(sym, prefix, depth, false)
			}
		}
		if imp.IsRecursive && !direct {
			nw.noImplicit--
		}
	}
}

// nameOccurrences counts how often qn names name.
func nameOccurrences(qn, name string) int {
	seen := 0
	for _, seg := range strings.Split(qn, ".") {
		if seg == name {
			seen++
		}
	}
	return seen
}

// add records one path and then the paths that lead through it.
func (nw *nameWalk) add(prefix, name string, sym *symbols.Symbol, depth int) {
	if name == "" || sym == nil || depth > nw.maxDepth {
		return
	}
	qn := name
	if prefix != "" {
		qn = prefix + "." + name
	}
	if nameOccurrences(qn, name) > maxNameOccurrences {
		return
	}
	target := nw.targetOf(sym)
	if nw.seen[qn] {
		if nw.reentry > 0 && nw.at[qn] != nil && nw.at[qn] != target && nw.loaded(target) {
			nw.expand(qn, target, depth+1)
		}
		return
	}
	nw.seen[qn] = true
	if !nw.loaded(target) {
		return
	}
	nw.at[qn] = target
	nw.out = append(nw.out, VisibleName{
		Name:  qn,
		FQN:   symbols.FQNOf(target),
		Kind:  target.Kind,
		Depth: depth,
	})
	nw.expand(qn, target, depth+1)
}

// expand adds the paths through a name: the public members of the element it
// reaches, which are what a qualified reference may name (KerML 8.2.3.3).
func (nw *nameWalk) expand(prefix string, sym *symbols.Symbol, depth int) {
	if depth > nw.maxDepth || nw.expanding[sym] >= maxNameOccurrences {
		return
	}
	// A path through an element ends where every type it would inherit from is
	// already on the path: what it declares there is not reachable again, but
	// the implicit members its own kind supplies still are.
	if sup := nw.sem.DirectSupertypes(sym); depth > 2 && len(sup) > 0 {
		blocked := 0
		for _, s := range sup {
			if nw.traversed[s] > 0 {
				blocked++
			}
		}
		if blocked == len(sup) {
			nw.implicitMembers(prefix, sym, depth)
			return
		}
	}
	nw.expanding[sym]++
	defer func() { nw.expanding[sym]-- }()

	// A declaration without a body carries no scope, and then only what it
	// inherits is reachable through it.
	if sym.Scope != nil {
		nw.locals(sym.Scope, prefix, depth, false, false)
	}
	nw.inherited(sym, prefix, depth, false)
	if sym.Scope != nil {
		nw.imports(sym.Scope, prefix, depth, false, false)
	}
}

// implicitMembers adds the members a blocked path still reaches: those whose
// declarer some chain of sources reaches without repeating a traversed type.
func (nw *nameWalk) implicitMembers(prefix string, sym *symbols.Symbol, depth int) {
	nw.expanding[sym]++
	defer func() { nw.expanding[sym]-- }()
	for _, m := range nw.membersOf(sym) {
		if !symbols.VisibleAs(m.Visibility, false, false) {
			continue
		}
		detour, ok := nw.chainAvoiding(sym, declarerOf(m))
		if !ok {
			continue
		}
		chain, ok := nw.enter(detour)
		if !ok {
			continue
		}
		nw.add(prefix, leafName(m.Name), m, depth)
		nw.add(prefix, m.ShortName, m, depth)
		nw.leave(chain)
	}
}

// chainAvoiding is chainTo restricted to sources the path has not gone through,
// reporting whether such a detour to declarer exists at all. Its first step
// also leaves out what a traversed general itself inherits from: the path has
// already inherited through that general, so it does not re-enter above it.
func (nw *nameWalk) chainAvoiding(owner, declarer *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if owner == nil || declarer == nil || declarer == owner {
		return nil, declarer != nil
	}
	above := map[*symbols.Symbol]bool{}
	for t, n := range nw.traversed {
		if n <= 0 {
			continue
		}
		for _, g := range nw.sem.DirectMemberSources(t) {
			above[g] = true
		}
	}
	from, found := nw.searchChain(owner, declarer, func(cur, src *symbols.Symbol) bool {
		return nw.traversed[src] > 0 || (cur == owner && above[src])
	})
	if !found {
		return nil, false
	}
	var chain []*symbols.Symbol
	for s := declarer; s != nil && s != owner; s = from[s] {
		chain = append(chain, s)
	}
	return chain, true
}

// loaded reports whether the caller's resource set holds an element: a library
// element counts only when its root package is named as loaded.
func (nw *nameWalk) loaded(sym *symbols.Symbol) bool {
	if !nw.idx.Library(sym) {
		return true
	}
	root := symbols.FQNOf(sym)
	if i := strings.Index(root, "::"); i >= 0 {
		root = root[:i]
	}
	return nw.library[root]
}

// importedName is the last name segment a membership import writes.
func importedName(imp *ast.Import) string {
	if imp.Imported == nil || len(imp.Imported.Parts) == 0 {
		return ""
	}
	return imp.Imported.Parts[len(imp.Imported.Parts)-1].Text
}

// leafName is a name without the qualification an indexed symbol carries.
func leafName(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[i+2:]
	}
	return name
}

// importsIn returns the imports declared directly in a namespace-bearing node.
func importsIn(node ast.Node) []*ast.Import {
	var members []ast.Node
	switch n := node.(type) {
	case *ast.Package:
		members = n.Members
	case *ast.Namespace:
		members = n.Members
	case *ast.RootNamespace:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	case *ast.Usage:
		members = n.Members
	default:
		return nil
	}
	var out []*ast.Import
	for _, m := range members {
		if imp, ok := m.(*ast.Import); ok {
			out = append(out, imp)
		}
	}
	return out
}

// ElementOnPath resolves a dotted path from a scope to the element it names:
// the first segment as an unqualified name, each later one as a member of the
// previous, an alias replaced by its target.
func (w *Workspace) ElementOnPath(scope *symbols.Scope, path []string) (*symbols.Symbol, bool) {
	if scope == nil || len(path) == 0 {
		return nil, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	var out *symbols.Symbol
	w.queryLocked(symbols.DocNameOf(scope), func(r *resolve.Resolver, sem *semantics.Model) {
		sym, ok := r.ResolveName(scope, path[0], nil)
		if !ok || sym == nil {
			return
		}
		for _, seg := range path[1:] {
			if sym, ok = sem.LookupMember(sym, seg); !ok || sym == nil {
				return
			}
		}
		if target, ok := r.ResolveAliasTarget(sym); ok && target != nil {
			sym = target
		}
		out = sym
	})
	return out, out != nil
}

// FQNOf is the qualified name the index registers an element under, which is
// how a caller comparing enumerations tells two paths to one element apart.
func (w *Workspace) FQNOf(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if fqn := w.index.GetFQN(sym); fqn != "" {
		return fqn
	}
	return sym.Name
}
