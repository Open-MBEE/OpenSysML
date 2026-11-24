package resolve

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// routeDeps is what an import read depended on of its route (KerML 8.3.2.4.5,
// 8.3.2.4.6): the excluded namespaces it was cut at, and the ones it entered.
type routeDeps struct {
	cut, entered []*symbols.Scope
}

// routeMark is a point in the record of cuts and entries (see routeDepsSince).
type routeMark struct {
	hits, seen, path int
}

// reading runs f with the namespace owning scope excluded, as the computation
// of its imported memberships excludes it.
func (r *Resolver) reading(scope *symbols.Scope, f func()) {
	if scope == nil || slices.Contains(r.importPath, scope) {
		f()
		return
	}
	depth, plain := len(r.importPath), r.plainRead
	if depth == 0 {
		r.plainRead = r.plainRegion(scope)
	}
	r.importPath = append(r.importPath, scope)
	defer func() {
		// Over a plain region a namespace read once stays excluded until the read that began ends.
		if depth == 0 || !r.plainRead {
			r.importPath = r.importPath[:depth]
		}
		r.plainRead = plain
		r.importNames = r.importNames[:min(len(r.importNames), len(r.importPath))]
		r.forgetRoute()
	}()
	f()
}

// entersNamespace reports whether a namespace import may read the namespace
// owning scope, which it may not while that namespace is excluded.
func (r *Resolver) entersNamespace(scope *symbols.Scope) bool {
	if scope == nil {
		return true
	}
	if slices.Contains(r.importPath, scope) {
		r.importHits = append(r.importHits, scope)
		return false
	}
	r.importSeen = append(r.importSeen, scope)
	return true
}

// freshImportRoute runs f as the read of a namespace in its own right: no
// namespace is excluded and no import is in progress, whatever route asked.
func (r *Resolver) freshImportRoute(f func()) {
	stack := r.importStack
	r.importStack = map[*ast.Import]bool{}
	defer func() { r.importStack = stack }()
	r.freshImportVisits(f)
}

// noNamespace stands in the excluded namespaces for a scope the index does
// not name: no namespace has it for a name, so it excludes none.
const noNamespace = "\x00"

// excludedNamespaces names the namespaces being read, as the index names them.
// The result is valid until the read that asked for it returns.
func (r *Resolver) excludedNamespaces() []string {
	for _, scope := range r.importPath[len(r.importNames):] {
		fqn := r.namespaceFQNOf(scope)
		if fqn == "" {
			fqn = noNamespace
		}
		r.importNames = append(r.importNames, fqn)
	}
	return r.importNames
}

// markRoute starts recording what the reads that follow depend on; each mark
// is closed by one routeDepsSince.
func (r *Resolver) markRoute() routeMark {
	r.importMarks++
	return routeMark{hits: len(r.importHits), seen: len(r.importSeen), path: len(r.importPath)}
}

// routeDepsSince closes mark: the namespaces excluded when it was set that the
// reads since were cut at, self apart, and the namespaces they entered.
func (r *Resolver) routeDepsSince(mark routeMark, self *symbols.Scope) routeDeps {
	entered := distinctScopes(r.importSeen[mark.seen:])
	return routeDeps{cut: r.routeCutsSince(mark, self), entered: entered}
}

// routeCutsSince closes mark for a read that depends on its cuts alone: the
// excluded namespaces still being read that the reads since were cut at, self
// apart.
func (r *Resolver) routeCutsSince(mark routeMark, self *symbols.Scope) []*symbols.Scope {
	var cut []*symbols.Scope
	for _, scope := range r.importHits[mark.hits:] {
		if scope != self && slices.Contains(r.importPath[:min(mark.path, len(r.importPath))], scope) && !slices.Contains(cut, scope) {
			cut = append(cut, scope)
		}
	}
	r.importMarks--
	r.forgetRoute()
	return cut
}

// forgetRoute drops the record once no read is in progress to depend on it.
func (r *Resolver) forgetRoute() {
	if r.importMarks == 0 && len(r.importPath) == 0 {
		r.importHits, r.importSeen = r.importHits[:0], r.importSeen[:0]
	}
}

// routeHolds reports whether an answer is the route in progress's too: its cuts
// recur and, for a set of members, every namespace it entered is readable.
func (r *Resolver) routeHolds(deps routeDeps, members bool) bool {
	for _, scope := range deps.cut {
		if !slices.Contains(r.importPath, scope) {
			return false
		}
	}
	if members {
		for _, scope := range r.importPath {
			if slices.Contains(deps.entered, scope) {
				return false
			}
		}
	}
	return true
}

// replayRoute makes the reads in progress depend on what a reused answer did.
func (r *Resolver) replayRoute(deps routeDeps) {
	r.importHits = append(r.importHits, deps.cut...)
	r.importSeen = append(r.importSeen, deps.entered...)
	r.forgetRoute()
}

func distinctScopes(scopes []*symbols.Scope) []*symbols.Scope {
	if len(scopes) == 0 {
		return nil
	}
	out := make([]*symbols.Scope, 0, len(scopes))
	if len(scopes) <= 8 {
		for _, scope := range scopes {
			if !slices.Contains(out, scope) {
				out = append(out, scope)
			}
		}
		return out
	}
	seen := make(map[*symbols.Scope]struct{}, len(scopes))
	for _, scope := range scopes {
		if _, dup := seen[scope]; !dup {
			seen[scope] = struct{}{}
			out = append(out, scope)
		}
	}
	return out
}

// plainRegion reports whether reading the imports of scope route by route (KerML 8.3.2.4.5)
// finds what one visit of each namespace does. Every namespace reachable is a package whose
// imports are plain namespace imports, each naming a package by owned members alone, none a
// route reads declares a filter, and no two members share a name: nothing can hide a name or read a namespace in two ways.
func (r *Resolver) plainRegion(scope *symbols.Scope) bool {
	if r.inCondition != 0 || !plainNamespace(scope) {
		return false
	}
	// Every read asks, so the tables are kept between calls; a call made within this one takes its own.
	region := r.plainWork
	r.plainWork = nil
	if region == nil {
		region = &plainRegionReader{r: r, named: map[string]*symbols.Symbol{}, seen: map[*symbols.Scope]bool{}}
	}
	defer func() {
		region.reset()
		r.plainWork = region
	}()
	region.scope = scope
	region.add(scope)
	if !region.reach() || len(region.seen) == 1 {
		return false
	}
	if len(region.filtered) != 0 && !region.filtersHold() {
		return false
	}
	return region.namesUnshadowed()
}

// plainImport is an import of the region read, with the namespace stating it, the member its
// first segment names and the package it names.
type plainImport struct {
	imp    *ast.Import
	from   *symbols.Scope
	named  *symbols.Symbol
	target *symbols.Symbol
}

// plainRegionReader reads the namespaces the imports of scope reach, for plainRegion.
type plainRegionReader struct {
	r     *Resolver
	scope *symbols.Scope
	// region holds the namespaces reached in the order found; those before read have been read.
	region []*symbols.Scope
	read   int
	seen   map[*symbols.Scope]bool
	// named holds the members of the namespaces read, by name.
	named   map[string]*symbols.Symbol
	imports []plainImport
	outside []symbols.OutsideImport
	// outer holds the namespaces around an import searched for its first segment; filtered,
	// the namespaces that import and declare a filter.
	outer, filtered []*symbols.Scope
	// routes holds the namespaces each namespace's imports lead to; stating, the namespaces
	// of other documents whose members were read beside an import they state.
	routes  map[*symbols.Scope][]*symbols.Scope
	stating map[*symbols.Scope]bool
}

// reset empties the tables, keeping their storage and no reference into a model.
func (p *plainRegionReader) reset() {
	clear(p.region)
	clear(p.imports)
	clear(p.outside)
	clear(p.outer)
	clear(p.filtered)
	p.region, p.imports, p.outside = p.region[:0], p.imports[:0], p.outside[:0]
	p.outer, p.filtered = p.outer[:0], p.filtered[:0]
	p.scope, p.read = nil, 0
	clear(p.seen)
	clear(p.named)
	clear(p.routes)
	clear(p.stating)
}

// unique reports whether no member of ns has the name of another read before it.
func (p *plainRegionReader) unique(ns *symbols.Scope) bool {
	distinct := true
	ns.ForEachNamedMember(func(name string, sym *symbols.Symbol) bool {
		if held, dup := p.named[name]; dup && held != sym {
			distinct = false
		}
		p.named[name] = sym
		return distinct
	})
	return distinct
}

func (p *plainRegionReader) add(ns *symbols.Scope) {
	if !p.seen[ns] {
		p.seen[ns] = true
		p.region = append(p.region, ns)
	}
}

// reach reads the namespaces of the region not read yet, and those their own imports reach.
func (p *plainRegionReader) reach() bool {
	for ; p.read < len(p.region); p.read++ {
		if !p.readNamespace(p.region[p.read]) {
			return false
		}
	}
	return true
}

func (p *plainRegionReader) readNamespace(ns *symbols.Scope) bool {
	stated := ns.Imports()
	if len(stated) != 0 && len(p.r.namespaceFilters(ns)) != 0 {
		p.filtered = append(p.filtered, ns)
	}
	if ns != p.scope && !(plainNamespace(ns) && p.unique(ns)) {
		return false
	}
	for _, imp := range stated {
		if !p.readImport(ns, imp) {
			return false
		}
	}
	return true
}

func (p *plainRegionReader) readImport(ns *symbols.Scope, imp *ast.Import) bool {
	if !plainImportOf(imp) {
		return false
	}
	// A lookup passes over an import whose target is being resolved; an enumeration reads it.
	if p.r.resolvingImports[imp] {
		return p.r.enumDepth <= 0
	}
	first, target, ok := ownedTarget(ns, imp, &p.outer)
	if !ok {
		return false
	}
	p.imports = append(p.imports, plainImport{imp: imp, from: ns, named: first, target: target})
	p.add(target.Scope)
	return true
}

// filtersHold reads the imports other documents state for the region's namespaces and
// reports whether each filter restricts what its namespace reaches on every route from scope.
func (p *plainRegionReader) filtersHold() bool {
	if p.routes == nil {
		p.routes = map[*symbols.Scope][]*symbols.Scope{}
		p.stating = map[*symbols.Scope]bool{}
	}
	root := rootOf(p.scope)
	// A namespace another document declares too has the imports stated there, each read as its own are.
	for at := 0; at < len(p.region); at++ {
		if !p.readStatedOutside(p.region[at], root) || !p.reach() {
			return false
		}
	}
	// Past scope a route takes public imports alone (KerML 8.3.2.4.5 visibleMemberships).
	for _, read := range p.imports {
		if read.from == p.scope || read.imp.Visibility != ast.VisibilityPrivate {
			p.routes[read.from] = append(p.routes[read.from], read.target.Scope)
		}
	}
	routed := reachedFrom(p.routes, p.scope, nil)
	for _, ns := range p.filtered {
		if routed[ns] && !filtersEveryRoute(p.routes, p.scope, ns) {
			return false
		}
	}
	return true
}

func (p *plainRegionReader) readStatedOutside(ns, root *symbols.Scope) bool {
	stated := p.r.importsStatedOutside(ns)
	if len(stated) != 0 && len(ns.Imports()) == 0 && len(p.r.namespaceFilters(ns)) != 0 {
		p.filtered = append(p.filtered, ns)
	}
	for _, stated := range stated {
		target := p.r.namespaceNamed(root, stated.Target)
		if target == nil || !plainImportOf(stated.Import) {
			return false
		}
		// The members stated beside the import are the namespace's too, unless it is the reader.
		if ns != p.scope && !p.stating[stated.From] {
			p.stating[stated.From] = true
			if !p.unique(stated.From) {
				return false
			}
		}
		p.outside = append(p.outside, stated)
		p.add(target)
		if ns == p.scope || stated.Import.Visibility != ast.VisibilityPrivate {
			p.routes[ns] = append(p.routes[ns], target)
		}
	}
	return true
}

// namesUnshadowed reports whether every import read names its target the way one visit does.
func (p *plainRegionReader) namesUnshadowed() bool {
	// A namespace around an import may bring names of its own only as one of these.
	for _, ns := range p.outer {
		if !p.seen[ns] && len(ns.Imports()) != 0 {
			return false
		}
	}
	for _, stated := range p.outside {
		// The index names its target from the root: no member read may answer to that name first.
		if _, had := p.named[stated.Import.Imported.Parts[0].Text]; had {
			return false
		}
	}
	for _, read := range p.imports {
		// The name an import starts at is no member's that an import could bring instead.
		if held, had := p.named[read.imp.Imported.Parts[0].Text]; had && held != read.named {
			return false
		}
		if target, ok := p.r.importTargetOf(read.from, read.imp); !ok || target != read.target {
			return false
		}
	}
	return true
}

// plainImportOf reports whether imp imports the visible members of a namespace and nothing else.
func plainImportOf(imp *ast.Import) bool {
	return imp.Kind == ast.ImportNamespace && !imp.IsAll && !imp.IsRecursive && imp.FilterExpr == nil && !imp.IsExpose
}

// importsStatedOutside are the imports the namespace owning scope is given by its declarations in other documents.
func (r *Resolver) importsStatedOutside(scope *symbols.Scope) []symbols.OutsideImport {
	fqn := r.namespaceFQNOf(scope)
	if fqn == "" {
		return nil
	}
	return r.idx.ImportsStatedOutside(fqn, r.documentOf(scope))
}

// namespaceNamed is the scope of the one namespace fqn names: the one the document of root declares,
// else the one another document does, or nil.
func (r *Resolver) namespaceNamed(root *symbols.Scope, fqn string) *symbols.Scope {
	if fqn == "" {
		return nil
	}
	at := root
	for name := range strings.SplitSeq(fqn, "::") {
		found := at.LookupLocalAll(name)
		if len(found) != 1 || found[0].Scope == nil {
			return r.idx.NamespaceAt(fqn)
		}
		at = found[0].Scope
	}
	return at
}

// reachedFrom is start and the namespaces the routes from it reach, none of them through stop.
func reachedFrom(routes map[*symbols.Scope][]*symbols.Scope, start, stop *symbols.Scope) map[*symbols.Scope]bool {
	reached := map[*symbols.Scope]bool{start: true}
	for pending := []*symbols.Scope{start}; len(pending) > 0; {
		ns := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, target := range routes[ns] {
			if target != stop && !reached[target] {
				reached[target] = true
				pending = append(pending, target)
			}
		}
	}
	return reached
}

// filtersEveryRoute reports whether every route from scope into what the imports of ns reach runs through
// ns: its `filter` member then restricts each of those namespaces on every route, as on one visit.
func filtersEveryRoute(routes map[*symbols.Scope][]*symbols.Scope, scope, ns *symbols.Scope) bool {
	if ns == scope {
		return true
	}
	// What scope reaches with ns left out is reached unfiltered.
	unfiltered := reachedFrom(routes, scope, ns)
	for target := range reachedFrom(routes, ns, nil) {
		if target != ns && unfiltered[target] {
			return false
		}
	}
	return true
}

// plainNamespace reports whether scope is a package declared in a document being analysed.
func plainNamespace(scope *symbols.Scope) bool {
	owner := scope.Owner()
	return owner != nil && owner.Kind == symbols.SymbolPackage && !scope.Recorded()
}

// ownedTarget resolves the package imp names from ns by owned members alone, with the member
// its first segment names; outer collects the enclosing namespaces searched for that segment.
func ownedTarget(ns *symbols.Scope, imp *ast.Import, outer *[]*symbols.Scope) (first, target *symbols.Symbol, ok bool) {
	if imp.Imported == nil || len(imp.Imported.Parts) == 0 || imp.Imported.Global {
		return nil, nil, false
	}
	parts := imp.Imported.Parts
	for at := ns; at != nil && first == nil; at = at.Parent() {
		if at != ns {
			*outer = append(*outer, at)
		}
		found := at.LookupLocalAll(parts[0].Text)
		if len(found) > 1 {
			return nil, nil, false
		}
		if len(found) == 1 {
			first = found[0]
		}
	}
	if first == nil {
		return nil, nil, false
	}
	target = first
	for _, part := range parts[1:] {
		if target.Scope == nil {
			return nil, nil, false
		}
		found := target.Scope.LookupLocalAll(part.Text)
		if len(found) != 1 || found[0].Visibility == ast.VisibilityPrivate {
			return nil, nil, false
		}
		target = found[0]
	}
	if target.Kind != symbols.SymbolPackage || target.Scope == nil {
		return nil, nil, false
	}
	return first, target, true
}
