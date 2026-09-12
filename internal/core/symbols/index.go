package symbols

import (
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// fqnSeparator joins the segments of a fully-qualified name.
const fqnSeparator = "::"

// fqnEntry records one symbol registered under a fully-qualified name.
type fqnEntry struct {
	fqn string
	sym *Symbol
}

// reexportKey is one re-export registration: a fully-qualified name and the
// symbol a wildcard import made reachable under it.
type reexportKey struct {
	fqn string
	sym *Symbol
}

// Index aggregates symbol information across all documents in a workspace.
// It owns each document's root scope and a global map from fully-qualified
// name to the symbol(s) declared under it. Per-document contributions are
// tracked so a document can be removed or re-added without leaving stale
// entries — the names it declared and the ones its wildcard imports surfaced
// alike.
//
// Each table is a layer (see layer.go): an index built with NewOverlay reads
// through to a frozen index's tables and writes only its own entries, so the
// standard library is indexed once and shared by every model rather than
// re-indexed per model.
type Index struct {
	// base is the frozen index this one reads through to, and frozen bars this
	// one from being written: a shared base must not change under the indexes
	// built over it.
	base                     *Index
	frozen                   bool
	generation               *indexGeneration
	directChildrenMu         sync.Mutex
	directChildrenGeneration uint64
	directChildrenCache      map[directChildrenKey][]*Symbol
	directChildrenByName     map[directChildrenKey]map[string][]*Symbol

	docRoots      *layer[string, *Scope]      // document name -> root scope
	docOfRoot     *layer[*Scope, string]      // root scope -> document name
	docKinds      *layer[string, source.Kind] // document name -> explicit language
	fqn           *layer[string, []*Symbol]   // fully-qualified name -> symbols
	contributions *layer[string, []fqnEntry]  // document name -> entries it added

	// wildcardMeta holds the wildcard imports of a namespace per document that
	// declares it, so removing a document stops its imports from being expanded
	// while the ones another document states through the same namespace survive.
	wildcardMeta *layer[string, map[string][]WildcardImport] // package FQN -> doc -> its wildcard imports

	// reexported marks the (FQN, symbol) pairs that a wildcard import made
	// visible rather than the namespace declaring them, so a lookup can prefer
	// the declared member. hidden is the subset a *private* import surfaced,
	// which a further wildcard import must not carry on. Both are derived from
	// reexportDocs and kept alongside it for the lookup path.
	reexported *layer[string, symbolSet]
	hidden     *layer[string, symbolSet]

	// reexportDocs attributes each re-export to every document whose wildcard
	// imports surface it, recording whether that document surfaces it publicly and
	// the element-filter conditions its imports impose; docReexports is the
	// per-document inverse. Two documents importing the same namespace surface the
	// same names, so removal drops one document's claim and keeps the registration
	// while another document still makes it.
	//
	// This is deliberately not folded into contributions: that is an append-only
	// slice per document, and a re-export has to be found by (FQN, symbol) to
	// drop one document's claim, or purged across all documents at once.
	reexportDocs *layer[reexportKey, map[string]*reexportClaim]
	docReexports *layer[string, map[reexportKey]bool]

	// declaredAt maps a symbol to the FQN its declaration gives it, which is the
	// only key its own members are registered under: a re-export registers the
	// symbol elsewhere but never copies its subtree.
	declaredAt *layer[*Symbol, string]

	// children maps a namespace's FQN to the keys registered directly under it,
	// so enumerating a wildcard import's members costs its members rather than a
	// scan of every name in the workspace.
	children *layer[string, []string]

	// bySegment maps a last name segment to the sorted qualified names ending in
	// it, so suggesting a candidate for an unresolved reference costs its matches
	// rather than a scan of every name the library declares.
	bySegment *layer[string, []string]

	// dirtyNS records how each namespace's direct members changed since the last
	// expansion, and lastTargets what each importer's imports resolved to when it
	// was last expanded — the routes its re-exports came by. Together they bound an
	// expansion to the importers a change can reach instead of the whole workspace.
	//
	// dirtyNS is not layered: a frozen base has settled, so an index over it
	// starts with nothing dirty and records only what its own documents changed.
	dirtyNS     map[string]nsChange
	lastTargets *layer[string, []resolvedImport]

	// libraryDocs names the documents that hold bundled library content rather
	// than the workspace's own, with the tier and text digest of each, and
	// librarySyms the symbols they declare, so a consumer can tell a library name
	// from one the user wrote and the frame from the libraries with semantics of
	// their own. Both are dropped with the document.
	libraryDocs *layer[string, LibraryDocument]
	librarySyms *layer[*Symbol, LibraryTier]

	// libraryIdentity memoizes LibraryIdentity as of the generation it was taken
	// at; a frozen index takes it once, when it freezes, and is not written after.
	libraryIdentity libraryIdentityMemo

	// nsFilters holds the element-filter conditions a namespace declares per
	// document that declares it, alongside wildcardMeta and for the same reason:
	// they restrict the memberships that namespace's imports bring in, so they are
	// read on every expansion of it.
	nsFilters *layer[string, map[string][]ElementFilter]

	// aboutUsages caches, per document and computed at Freeze, the metadata
	// usages with an `about` clause the document declares, so a model over a
	// shared frozen index reads them instead of walking its scope trees.
	aboutUsages map[string][]*Symbol
}

// reexportClaim is one document's claim on a re-export: whether its imports
// surface the name publicly, and the element-filter conditions they impose on
// it. Conditions are recorded rather than applied here — judging a candidate
// needs the semantic model, which the index has no access to — and are read back
// by the resolver, which evaluates them (see ReexportGates).
//
// A name can be surfaced by more than one import, so the conditions are held per
// route: the element is a member if *any* route admits it, which is what makes an
// unfiltered import of the same namespace re-export it regardless of another
// route's filter. A zero-length route is one no filter restricts.
//
// Holding the routes with the claim that produced them is what keeps them exact:
// dropping a document's claim drops its routes, so a surviving document's filter
// is not defeated by a route that no longer exists.
type reexportClaim struct {
	public bool
	routes []gateRoute
}

// gateRoute is one route a name reached a namespace by: the conditions it
// imposes, and whether the import that made it was private, which keeps the
// route out of a lookup made from outside that namespace (KerML 8.2.3.3) —
// a private unfiltered import must not defeat a public filtered one.
type gateRoute struct {
	private bool
	filters []ElementFilter
}

// nsChange is what happened to a namespace's direct members since the last
// expansion. Gaining members can only add re-exports downstream; losing one, or
// having one hidden again, can invalidate them.
type nsChange struct {
	gained bool
	lost   bool
}

type directChildrenKey struct {
	prefix        string
	allowsPrivate bool
}

// WildcardImport is one `import X::*` declaration: the target's raw qualified
// name text, whether the import was declared private, and the filter condition
// restricting what it brings in (`import X::*[@Safety]`), which is zero for an
// unfiltered import.
type WildcardImport struct {
	Target  string
	Private bool
	Filter  ElementFilter
}

// NewIndex creates an empty index.
func NewIndex() *Index {
	gen := &indexGeneration{}
	return &Index{
		generation:           gen,
		directChildrenCache:  make(map[directChildrenKey][]*Symbol),
		directChildrenByName: make(map[directChildrenKey]map[string][]*Symbol),
		docRoots:             newLayer[string, *Scope](gen),
		docOfRoot:            newLayer[*Scope, string](gen),
		docKinds:             newLayer[string, source.Kind](gen),
		fqn:                  newLayer[string, []*Symbol](gen),
		contributions:        newLayer[string, []fqnEntry](gen),
		wildcardMeta:         newLayer[string, map[string][]WildcardImport](gen),
		reexported:           newLayer[string, symbolSet](gen),
		hidden:               newLayer[string, symbolSet](gen),
		reexportDocs:         newLayer[reexportKey, map[string]*reexportClaim](gen),
		docReexports:         newLayer[string, map[reexportKey]bool](gen),
		declaredAt:           newLayer[*Symbol, string](gen),
		children:             newLayer[string, []string](gen),
		bySegment:            newLayer[string, []string](gen),
		dirtyNS:              make(map[string]nsChange),
		lastTargets:          newLayer[string, []resolvedImport](gen),
		libraryDocs:          newLayer[string, LibraryDocument](gen),
		librarySyms:          newLayer[*Symbol, LibraryTier](gen),
		nsFilters:            newLayer[string, map[string][]ElementFilter](gen),
	}
}

// Freeze settles the index's wildcard imports and bars it from further writes,
// after which NewOverlay may build indexes over it. Freezing what holds the
// standard library is what lets every model share one copy of it.
func (idx *Index) Freeze() {
	if idx.frozen {
		return
	}
	idx.ExpandWildcardImports()
	idx.aboutUsages = make(map[string][]*Symbol)
	for _, name := range idx.docRoots.keys() {
		if usages := aboutUsagesIn(idx.docRoots.at(name)); len(usages) > 0 {
			idx.aboutUsages[name] = usages
		}
	}
	idx.takeLibraryIdentity()
	idx.frozen = true
}

// FrozenAboutUsages returns the `about` metadata usages the named document
// declares, read from the cache its frozen index built, and whether the cache
// covers the document: false for one still writable, whose tree the caller
// must walk instead.
func (idx *Index) FrozenAboutUsages(name string) ([]*Symbol, bool) {
	if idx.frozen {
		return idx.aboutUsages[name], true
	}
	if idx.base != nil && !idx.docRoots.owns(name) {
		if _, visible := idx.docRoots.get(name); visible {
			return idx.base.aboutUsages[name], true
		}
	}
	return nil, false
}

// aboutUsagesIn walks a scope tree — anonymous members included — collecting
// every metadata usage that states what it annotates with an `about` clause.
func aboutUsagesIn(root *Scope) []*Symbol {
	var out []*Symbol
	collectAboutUsages(root, make(map[*Symbol]bool), &out)
	return out
}

func collectAboutUsages(scope *Scope, seen map[*Symbol]bool, out *[]*Symbol) {
	if scope == nil {
		return
	}
	scope.ForEachMember(func(sym *Symbol) bool {
		if sym == nil || seen[sym] {
			return true
		}
		seen[sym] = true
		if sym.Kind == SymbolMetadataUsage {
			if usage, ok := sym.Decl.(*ast.Usage); ok && UsageAnnotatesOthers(usage) {
				*out = append(*out, sym)
			}
		}
		collectAboutUsages(sym.Scope, seen, out)
		return true
	})
}

// UsageAnnotatesOthers reports whether a metadata usage states what it
// annotates (`metadata m about p;`), rather than annotating its owner.
func UsageAnnotatesOthers(u *ast.Usage) bool {
	for _, rel := range u.Relationships {
		if rel != nil && rel.Kind == ast.RelAnnotates {
			return true
		}
	}
	return false
}

// Frozen reports whether the index has been frozen.
func (idx *Index) Frozen() bool { return idx.frozen }

// NewOverlay returns an index holding everything base holds, whose own writes
// are its own: documents it adds, the re-exports their imports surface and the
// ones they invalidate are visible in it alone, and base is left untouched. It
// panics if base is not frozen, since an index cannot read through to a table
// still being written.
func NewOverlay(base *Index) *Index {
	if base == nil || !base.frozen {
		panic("symbols: NewOverlay needs a frozen base index")
	}
	if base.base != nil {
		panic("symbols: NewOverlay cannot stack over an overlay")
	}
	gen := &indexGeneration{}
	return &Index{
		base:                 base,
		generation:           gen,
		directChildrenCache:  make(map[directChildrenKey][]*Symbol),
		directChildrenByName: make(map[directChildrenKey]map[string][]*Symbol),
		docRoots:             overLayer(base.docRoots, gen),
		docOfRoot:            overLayer(base.docOfRoot, gen),
		docKinds:             overLayer(base.docKinds, gen),
		fqn:                  overLayer(base.fqn, gen),
		contributions:        overLayer(base.contributions, gen),
		wildcardMeta:         overLayer(base.wildcardMeta, gen),
		reexported:           overLayer(base.reexported, gen),
		hidden:               overLayer(base.hidden, gen),
		reexportDocs:         overLayer(base.reexportDocs, gen),
		docReexports:         overLayer(base.docReexports, gen),
		declaredAt:           overLayer(base.declaredAt, gen),
		children:             overLayer(base.children, gen),
		bySegment:            overLayer(base.bySegment, gen),
		dirtyNS:              make(map[string]nsChange),
		lastTargets:          overLayer(base.lastTargets, gen),
		libraryDocs:          overLayer(base.libraryDocs, gen),
		librarySyms:          overLayer(base.librarySyms, gen),
		nsFilters:            overLayer(base.nsFilters, gen),
	}
}

// mustBeWritable stops a write to a frozen index: the indexes built over it read
// its tables, so a change to one of them would be a change to all of them.
func (idx *Index) mustBeWritable(op string) {
	if idx.frozen {
		panic("symbols: " + op + " on a frozen index")
	}
}

// AddDocument builds the scope tree for root and records its symbols under
// their fully-qualified names. Re-adding the same document name first removes
// the document's previous contributions, so the index stays exact.
//
// The names the document's wildcard imports surface are added by
// ExpandWildcardImports, which the caller runs once the documents it wants
// indexed are in: adding a document cannot know whether the target of an import
// it states is still to come.
func (idx *Index) AddDocument(name string, root *ast.RootNamespace) {
	idx.addDocument(name, root, source.KindOf(name), false)
}

// AddDocumentWithKind builds the scope tree for root and records its explicit
// language, which is needed when the document name does not carry an extension.
func (idx *Index) AddDocumentWithKind(name string, root *ast.RootNamespace, kind source.Kind) {
	idx.addDocument(name, root, kind, true)
}

func (idx *Index) addDocument(name string, root *ast.RootNamespace, kind source.Kind, explicitKind bool) {
	idx.mustBeWritable("AddDocument")
	idx.RemoveDocument(name)
	rs := Build(root)
	SetDocName(rs, name)
	idx.docRoots.set(name, rs)
	if explicitKind {
		idx.docKinds.set(name, kind)
	}
	idx.docOfRoot.set(rs, name)
	idx.indexScope(name, rs, "")

	// Extract wildcard imports and filters from the root namespace itself
	// (root is not a symbol, so indexScope won't process its members)
	if wildcards := extractWildcardImports(root, rs); len(wildcards) > 0 {
		idx.setWildcardImports("", name, wildcards)
	}
	idx.SetNamespaceFilters("", name, extractNamespaceFilters(root, rs))
}

// setWildcardImports records the wildcard imports doc states through the
// namespace registered under pkgFQN, and marks that namespace for expansion.
func (idx *Index) setWildcardImports(pkgFQN, doc string, imports []WildcardImport) {
	writableMap(idx.wildcardMeta, pkgFQN)[doc] = imports
	idx.lastTargets.del(pkgFQN) // its import set changed: expand it again
}

// ExpandWildcardImports adds re-exported symbols for every package with a
// wildcard import like `import ISQMechanics::*`, making the target's members
// visible through the importing package's FQN. Call it after the documents to
// index are in; AddDocument and AddRecords do not expand on their own, while
// RemoveDocument does, so the index a removal leaves is the one a fresh build
// over the remaining documents would produce.
//
// Imports chain — KerML imports Kernel::*, which imports Core::*, which imports
// Root::* — so a single pass would only propagate one level and its result
// would depend on the order the importing packages happened to be visited in.
// Passes therefore repeat until nothing new is re-exported, over the importers
// in name order, which makes the outcome independent of both map iteration
// order and of whether a document was parsed or restored from cache.
//
// A pass touches only the importers a change since the last expansion can
// reach: one whose imports changed, one whose import now resolves to a different
// namespace, and one importing a namespace whose members changed. Calling it
// when nothing changed costs one resolution of each import and registers
// nothing, so a caller that expands after every edit pays for the edit rather
// than for the workspace.
//
// An importer whose imports no longer support what it re-exported has those
// re-exports dropped before the pass derives again, since a re-export can be
// reached by more than one route and a cycle of imports would otherwise let one
// support itself. Dropping propagates: it takes members from a namespace, which
// is a change the importers of *that* namespace see on the next pass.
func (idx *Index) ExpandWildcardImports() {
	idx.mustBeWritable("ExpandWildcardImports")
	for round := 0; round < expansionRounds; round++ {
		if !idx.expandRound(false) {
			return
		}
	}
	// A change whose re-derivation keeps invalidating itself would not settle.
	// Deriving every importer from an empty re-export state is the computation a
	// fresh build performs: it only ever adds, so it settles.
	idx.purgeAllReexports()
	for idx.expandRound(true) {
	}
}

// expansionRounds bounds the incremental purge-and-derive rounds before
// expansion falls back to deriving every importer from scratch. Reaching it
// costs a full re-derivation and changes nothing about the result.
const expansionRounds = 16

// expandRound brings the importers a change can have reached up to date and
// reports whether it changed anything. With deriveOnly set it never drops a
// re-export, which is what a build from an empty re-export state needs.
func (idx *Index) expandRound(deriveOnly bool) bool {
	changed := idx.dirtyNS
	idx.dirtyNS = make(map[string]nsChange)
	purge, derive := idx.importersToRefresh(changed, deriveOnly)
	if len(purge) == 0 && len(derive) == 0 {
		return false
	}
	// Every purge lands before any derivation, so that an importer deriving from
	// another cannot copy re-exports that are about to be dropped.
	for _, pkgFQN := range purge {
		idx.purgeReexportsUnder(pkgFQN)
		idx.lastTargets.del(pkgFQN)
	}
	for _, pkgFQN := range derive {
		idx.expandImporter(pkgFQN)
	}
	return true
}

// importersToRefresh splits the importers a change can have reached into the
// ones whose re-exports no longer follow from their imports, and the ones to
// derive — the former plus those importing a namespace that gained members. Both
// are in name order, so the outcome does not depend on map iteration order.
func (idx *Index) importersToRefresh(changed map[string]nsChange, deriveOnly bool) (purge, derive []string) {
	pkgFQNs := idx.wildcardMeta.keys()
	sort.Strings(pkgFQNs)

	for _, pkgFQN := range pkgFQNs {
		now := idx.resolveImports(pkgFQN)
		last, expanded := idx.lastTargets.get(pkgFQN)
		if !expanded {
			derive = append(derive, pkgFQN)
			continue
		}
		// It is stale when an import names another namespace than it did, or when a
		// namespace it read members from — then or now — lost some.
		stale := !sameImports(last, now) ||
			lostMembers(changed, last) || lostMembers(changed, now)
		if stale {
			if !deriveOnly {
				purge = append(purge, pkgFQN)
			}
			derive = append(derive, pkgFQN)
			continue
		}
		for _, imp := range now {
			if changed[imp.fqn].gained {
				derive = append(derive, pkgFQN)
				break
			}
		}
	}
	return purge, derive
}

// statedImport is one wildcard import as declared: the document stating it, its
// unresolved target text, and its declared visibility.
type statedImport struct {
	doc     string
	target  string
	private bool
	filter  ElementFilter
}

// resolvedImport is one wildcard import paired with the FQN its target resolves
// to ("" when the target is unknown or ambiguous).
type resolvedImport struct {
	doc     string
	fqn     string
	private bool
}

// imports returns every wildcard import stated through pkgFQN, over the
// documents in name order so the result does not depend on map iteration order.
func (idx *Index) imports(pkgFQN string) []statedImport {
	byDoc := idx.wildcardMeta.at(pkgFQN)
	docs := make([]string, 0, len(byDoc))
	for doc := range byDoc {
		docs = append(docs, doc)
	}
	sort.Strings(docs)

	var out []statedImport
	for _, doc := range docs {
		for _, imp := range byDoc[doc] {
			out = append(out, statedImport{doc: doc, target: imp.Target, private: imp.Private, filter: imp.Filter})
		}
	}
	return out
}

// resolveImports pairs each import stated through pkgFQN with the FQN its target
// resolves to against the index as it stands.
func (idx *Index) resolveImports(pkgFQN string) []resolvedImport {
	stated := idx.imports(pkgFQN)
	out := make([]resolvedImport, len(stated))
	for i, imp := range stated {
		out[i] = resolvedImport{
			doc:     imp.doc,
			fqn:     idx.resolveWildcardTarget(pkgFQN, imp.target),
			private: imp.private,
		}
	}
	return out
}

// lostMembers reports whether any namespace these imports name lost a member,
// or had one hidden again, since the last expansion.
func lostMembers(changed map[string]nsChange, imports []resolvedImport) bool {
	for _, imp := range imports {
		if changed[imp.fqn].lost {
			return true
		}
	}
	return false
}

// sameImports reports whether two resolutions of a namespace's imports agree, so
// an expansion can tell that an import now names a different namespace, or none.
func sameImports(a, b []resolvedImport) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// expandImporter re-exports the visible members of every namespace pkgFQN's
// imports name, attributing each to the document whose import surfaced it.
//
// Each target is resolved as its import is reached, not up front: an import can
// name what an earlier one in the same namespace brought in, as P's
// `import Shared::*` names the P::Shared that its `import Outer::*` re-exported.
func (idx *Index) expandImporter(pkgFQN string) {
	for _, imp := range idx.imports(pkgFQN) {
		targetFQN := idx.resolveWildcardTarget(pkgFQN, imp.target)
		if targetFQN == "" {
			continue // target not found or ambiguous
		}
		// The conditions this import adds: its own filter clause, and the `filter`
		// members of the namespace it imports into, which restrict that namespace's
		// imported memberships (KerML 8.2.4). They gate the re-export rather than
		// suppressing it, because whether a candidate satisfies them is a question
		// only the semantic model can answer.
		gate := nonZeroFilters(append([]ElementFilter{imp.filter}, idx.namespaceFiltersGating(pkgFQN, imp.doc)...))
		direct := [][]ElementFilter{gate}
		for _, child := range idx.exportedChildren(targetFQN) {
			// Extract child's primary name
			childName := lastSegment(child.Name)
			idx.reexportGated(joinFQN(pkgFQN, childName), child, imp.doc, imp.private,
				idx.routesOnward(imp.doc, targetFQN, childName, child, direct))

			// Also re-export under short name if different from primary name
			if child.ShortName != "" && child.ShortName != childName {
				idx.reexportGated(joinFQN(pkgFQN, child.ShortName), child, imp.doc, imp.private,
					idx.routesOnward(imp.doc, targetFQN, child.ShortName, child, direct))
			}
		}
	}
	idx.lastTargets.set(pkgFQN, idx.resolveImports(pkgFQN))
}

// resolveWildcardTarget resolves a wildcard import target name to the
// fully-qualified name it names. Handles both absolute references
// (ISQMechanics) and references relative to the importing package (Systems
// within SysML). Returns "" if the target is unknown or ambiguous.
//
// The answer is the FQN the target was declared under, not the matched symbol's
// Name: a symbol built from a parsed document carries only its local name,
// while one restored from a cache record carries its fully-qualified one.
//
// A relative target is searched from the importing package outward through its
// enclosing packages before the global namespace, as KerML 8.2.3.5 resolves a
// name: KerML::Core's `import Root::*` names its sibling KerML::Root.
func (idx *Index) resolveWildcardTarget(pkgFQN, targetText string) string {
	for prefix := pkgFQN; prefix != ""; {
		if fqn, ok := idx.wildcardTargetAt(prefix + "::" + targetText); ok {
			return fqn
		}
		i := lastSeparator(prefix)
		if i < 0 {
			break
		}
		prefix = prefix[:i]
	}

	// Global namespace
	if fqn, ok := idx.wildcardTargetAt(targetText); ok {
		return fqn
	}

	// Target not found or ambiguous
	return ""
}

// wildcardTargetAt reports the FQN a wildcard import reads its members from
// when key names exactly one namespace, and whether it does.
//
// A namespace declared under key holds its members there, and shadows anything
// a wildcard import also re-exported under that name (SI::min is SI's minute).
// Either way the answer is the FQN the symbol was declared under, the only key
// its members are registered under: neither a re-export nor the short-name entry
// of `package <USCU> USCustomaryUnits` copies its subtree.
func (idx *Index) wildcardTargetAt(key string) (string, bool) {
	imported := idx.reexported.at(key)
	owned, reexports := 0, 0
	var soleOwned, soleImported *Symbol
	for _, sym := range idx.fqn.at(key) {
		if imported.has(sym) {
			reexports++
			soleImported = sym
			continue
		}
		owned++
		soleOwned = sym
	}
	switch {
	case owned == 1:
		if declared, ok := idx.declaredAt.get(soleOwned); ok {
			return declared, true
		}
		return key, true
	case owned == 0 && reexports == 1:
		declared, ok := idx.declaredAt.get(soleImported)
		return declared, ok
	default:
		return "", false // unknown, or ambiguous between namespaces
	}
}

// register records sym under fqn, linking fqn to its parent namespace — the
// document root "" included, so a file-level import's re-exports can be dropped.
func (idx *Index) register(fqn string, sym *Symbol) {
	idx.link(fqn, sym)
	parent, _ := splitFQN(fqn)
	idx.markGained(parent)
}

// link is register without noting the change to the parent namespace, for a
// caller that records it itself.
func (idx *Index) link(fqn string, sym *Symbol) {
	appendSlice(idx.fqn, fqn, sym)
	parent, last := splitFQN(fqn)
	insertSorted(idx.children, parent, fqn)
	if parent != "" {
		insertSorted(idx.bySegment, last, fqn)
	}
}

// unregister drops fqn once no symbol is registered under it.
func (idx *Index) unregister(fqn string) {
	parent, _ := splitFQN(fqn)
	if _, ok := idx.children.get(parent); !ok {
		return
	}
	if kids := removeSorted(idx.children, parent, fqn); len(kids) == 0 {
		idx.children.del(parent)
	}
}

// unregisterSegment forgets fqn under its last name segment, once nothing is
// registered under it.
func (idx *Index) unregisterSegment(fqn string) {
	parent, last := splitFQN(fqn)
	if parent == "" {
		return
	}
	if _, ok := idx.bySegment.get(last); !ok {
		return
	}
	if names := removeSorted(idx.bySegment, last, fqn); len(names) == 0 {
		idx.bySegment.del(last)
	}
}

// deregister drops sym from the symbols registered under fqn, forgetting fqn
// entirely once it names nothing. It leaves declaredAt alone: only the symbol's
// own declaration owns that entry.
func (idx *Index) deregister(fqn string, sym *Symbol) {
	syms := writableSlice(idx.fqn, fqn)
	for i, s := range syms {
		if s == sym {
			syms = append(syms[:i], syms[i+1:]...)
			break
		}
	}
	if len(syms) == 0 {
		idx.fqn.del(fqn)
		idx.reexported.del(fqn)
		idx.hidden.del(fqn)
		idx.unregister(fqn)
		idx.unregisterSegment(fqn)
	} else {
		idx.fqn.set(fqn, syms)
		clearMark(idx.reexported, fqn, sym)
		clearMark(idx.hidden, fqn, sym)
	}
	parent, _ := splitFQN(fqn)
	idx.markLost(parent)
}

// markGained and markLost record how a namespace's direct members changed, which
// is what the next expansion reads to find the importers a change reached.
func (idx *Index) markGained(ns string) {
	change := idx.dirtyNS[ns]
	change.gained = true
	idx.dirtyNS[ns] = change
}

func (idx *Index) markLost(ns string) {
	change := idx.dirtyNS[ns]
	change.lost = true
	idx.dirtyNS[ns] = change
}

// splitFQN separates fqn into its owning namespace and the name within it.
func splitFQN(fqn string) (parent, name string) {
	i := lastSeparator(fqn)
	if i < 0 {
		return "", fqn
	}
	return fqn[:i], fqn[i+2:]
}

// lastSeparator returns the index of the last "::" in fqn, or -1. It is on the
// path of every re-export, where strings.LastIndex's general search costs.
func lastSeparator(fqn string) int {
	for i := len(fqn) - 2; i >= 0; i-- {
		if fqn[i] == ':' && fqn[i+1] == ':' {
			return i
		}
	}
	return -1
}

func (idx *Index) hasFQN(fqn string, sym *Symbol) bool {
	for _, s := range idx.fqn.at(fqn) {
		if s == sym {
			return true
		}
	}
	return false
}

// RemoveDocument drops all of the named document's contributions from the
// global index and forgets its root scope: the names it declared, the wildcard
// imports it stated, and the re-exports those imports surfaced. A re-export a
// surviving document also surfaces stays, as does the declaredAt entry a
// surviving declaration owns.
//
// Removal re-expands what it invalidated before returning, so the index it
// leaves is the one a fresh build over the remaining documents produces — a
// caller does not have to know that removing a document can break a chain of
// imports (A imports B imports C) or make an ambiguous import target resolvable.
// Unknown names are a no-op.
//
// An index built over a frozen base may remove one of the base's documents too:
// the removal is recorded in the overlay, which stops answering for what the
// document contributed while the base keeps it for every other index over it.
func (idx *Index) RemoveDocument(name string) {
	idx.mustBeWritable("RemoveDocument")
	if !idx.knows(name) {
		return
	}

	library := idx.libraryDocs.at(name).Tier.Library()
	for _, e := range idx.contributions.at(name) {
		if idx.declaredAt.at(e.sym) == e.fqn {
			idx.declaredAt.del(e.sym)
			if library {
				idx.librarySyms.del(e.sym)
			}
		}
		idx.deregister(e.fqn, e.sym)
	}
	idx.libraryDocs.del(name)
	idx.docKinds.del(name)
	idx.contributions.del(name)
	if root, ok := idx.docRoots.get(name); ok {
		idx.docOfRoot.del(root)
	}
	idx.docRoots.del(name)

	for _, pkgFQN := range idx.wildcardMeta.keys() {
		if _, ok := idx.wildcardMeta.at(pkgFQN)[name]; !ok {
			continue
		}
		byDoc := writableMap(idx.wildcardMeta, pkgFQN)
		delete(byDoc, name)
		if len(byDoc) == 0 {
			idx.wildcardMeta.del(pkgFQN)
		}
		idx.lastTargets.del(pkgFQN) // its import set changed: expand it again
	}

	for key := range idx.docReexports.at(name) {
		idx.dropClaim(key, name)
	}
	idx.docReexports.del(name)
	idx.dropNamespaceFilters(name)

	idx.ExpandWildcardImports()
}

// MarkLibrary records that the named document holds bundled library content,
// not the workspace's own, of no stated tier. Call it after the document is
// added: adding a document removes its previous contributions, and the mark
// with them. Only what the document declares is marked, not the names it
// re-exports.
func (idx *Index) MarkLibrary(name string) {
	idx.MarkLibraryTier(name, TierLibrary)
}

// MarkLibraryTier records that the named document holds bundled library content
// of the given tier and unstated text; TierNone unmarks it. See MarkLibrary for
// when to call it.
func (idx *Index) MarkLibraryTier(name string, tier LibraryTier) {
	idx.MarkLibraryDocument(name, LibraryDocument{Tier: tier})
}

// MarkLibraryDocument records that the named document holds the described
// bundled library content; a document of TierNone is unmarked. See MarkLibrary
// for when to call it.
func (idx *Index) MarkLibraryDocument(name string, doc LibraryDocument) {
	idx.mustBeWritable("MarkLibraryDocument")
	if doc.Tier == TierNone {
		idx.libraryDocs.del(name)
	} else {
		idx.libraryDocs.set(name, doc)
	}
	for _, e := range idx.contributions.at(name) {
		if idx.declaredAt.at(e.sym) != e.fqn {
			continue
		}
		if doc.Tier == TierNone {
			idx.librarySyms.del(e.sym)
		} else {
			idx.librarySyms.set(e.sym, doc.Tier)
		}
	}
}

// LibraryIdentity digests the library content the index holds — every library
// document's name, tier and text — so two indexes that agree on it declare the
// same library. It is unknown while a library document states no text digest.
func (idx *Index) LibraryIdentity() (string, bool) {
	if !idx.frozen && (!idx.libraryIdentity.taken || idx.libraryIdentity.gen != idx.generation.get()) {
		idx.takeLibraryIdentity()
	}
	return idx.libraryIdentity.value, idx.libraryIdentity.known
}

// libraryIdentityMemo is LibraryIdentity as of the index generation it was taken at.
type libraryIdentityMemo struct {
	taken bool
	gen   uint64
	value string
	known bool
}

func (idx *Index) takeLibraryIdentity() {
	docs := make(map[string]LibraryDocument)
	for _, name := range idx.libraryDocs.keys() {
		docs[name] = idx.libraryDocs.at(name)
	}
	idx.libraryIdentity.value, idx.libraryIdentity.known = libraryIdentityOf(docs)
	idx.libraryIdentity.gen = idx.generation.get()
	idx.libraryIdentity.taken = true
}

// HasLibrary reports whether the index holds any bundled library document.
func (idx *Index) HasLibrary() bool {
	return len(idx.libraryDocs.keys()) > 0
}

// Library reports whether sym is declared by bundled library content.
func (idx *Index) Library(sym *Symbol) bool {
	return idx.LibraryTier(sym).Library()
}

// LibraryTier reports the tier of the bundled library content that declares
// sym, TierNone for a symbol the workspace declares.
func (idx *Index) LibraryTier(sym *Symbol) LibraryTier {
	if sym == nil {
		return TierNone
	}
	return idx.librarySyms.at(sym)
}

// DocumentLibraryTier reports the tier the named document was marked with,
// TierNone for a workspace document.
func (idx *Index) DocumentLibraryTier(name string) LibraryTier {
	return idx.libraryDocs.at(name).Tier
}

// LibraryDocumentOf reports what the named document was marked as holding, a
// LibraryDocument of TierNone for a workspace document.
func (idx *Index) LibraryDocumentOf(name string) LibraryDocument {
	return idx.libraryDocs.at(name)
}

// IsLibraryDocument reports whether the named document holds bundled library
// content (see MarkLibrary).
func (idx *Index) IsLibraryDocument(name string) bool {
	return idx.DocumentLibraryTier(name).Library()
}

// knows reports whether the index holds anything for the named document.
func (idx *Index) knows(name string) bool {
	if _, ok := idx.contributions.get(name); ok {
		return true
	}
	if _, ok := idx.docRoots.get(name); ok {
		return true
	}
	if _, ok := idx.docReexports.get(name); ok {
		return true
	}
	for _, pkgFQN := range idx.wildcardMeta.keys() {
		if _, ok := idx.wildcardMeta.at(pkgFQN)[name]; ok {
			return true
		}
	}
	return false
}

// purgeAllReexports drops every re-export in the index, leaving the names the
// documents declare.
func (idx *Index) purgeAllReexports() {
	for _, key := range idx.reexportDocs.keys() {
		idx.purgeReexport(key)
	}
	idx.lastTargets.clear()
}

// purgeReexportsUnder drops every re-export registered directly under pkgFQN,
// whichever documents claim it. What the surviving documents' imports still
// support is re-derived by the following expansion.
func (idx *Index) purgeReexportsUnder(pkgFQN string) {
	for _, fqn := range idx.childKeys(pkgFQN) {
		for _, sym := range idx.reexportedAt(fqn) {
			idx.purgeReexport(reexportKey{fqn: fqn, sym: sym})
		}
	}
}

// reexportedAt returns the symbols a wildcard import surfaced under fqn. The
// copy lets callers deregister while they iterate.
func (idx *Index) reexportedAt(fqn string) []*Symbol {
	return slices.Clone(idx.reexported.at(fqn))
}

// reexport registers sym under fqn on behalf of a wildcard import doc states,
// recording the claim so that removing doc can take it back. An entry the
// importing namespace declares itself is left alone: a cycle of wildcard imports
// brings a package its own members back, and they are not borrowed.
// routesOnward composes the conditions already gating a name inside the
// namespace it is imported from — sourceFQN::name, where an earlier import
// surfaced it — with the ones this import adds. A filter therefore keeps holding
// when a further namespace imports the filtering one onward, and the target's
// several routes each stay a route of their own. direct holds this import's
// gate alone, the routes of a name nothing gated on the way in.
func (idx *Index) routesOnward(doc, sourceFQN, name string, sym *Symbol, direct [][]ElementFilter) [][]ElementFilter {
	if idx.declaredExactly(sym, sourceFQN, name) {
		return direct // declared there, never borrowed into it
	}
	// The importing namespace is outside the one it imports from, so only that
	// one's public routes reach it.
	inherited := idx.ReexportGates(doc, joinFQN(sourceFQN, name), sym, "")
	if len(inherited) == 0 {
		return direct
	}
	gate := direct[0]
	if len(gate) == 0 {
		return inherited // an unfiltered import passes the routes on as they are
	}
	out := make([][]ElementFilter, 0, len(inherited))
	for _, route := range inherited {
		out = append(out, addFilters(route, gate))
	}
	return out
}

// declaredExactly reports whether sym's declaration registered it as
// prefix::name, in which case no re-export claim exists under that key.
func (idx *Index) declaredExactly(sym *Symbol, prefix, name string) bool {
	declared, ok := idx.declaredAt.get(sym)
	if !ok {
		return false
	}
	if prefix == "" {
		return declared == name
	}
	return len(declared) == len(prefix)+2+len(name) &&
		declared[:len(prefix)] == prefix &&
		declared[len(prefix):len(prefix)+2] == "::" &&
		declared[len(prefix)+2:] == name
}

// addFilters composes two routes' conditions, dropping one the route already
// carries: conditions apply conjunctively, so repeating one adds nothing — and a
// cycle of filtered imports would otherwise compose forever.
func addFilters(route, add []ElementFilter) []ElementFilter {
	out := append([]ElementFilter{}, route...)
	for _, f := range add {
		if !hasFilter(out, f) {
			out = append(out, f)
		}
	}
	return out
}

// hasFilter reports whether a route already carries the condition f.
func hasFilter(route []ElementFilter, f ElementFilter) bool {
	for _, have := range route {
		if have.Same(f) {
			return true
		}
	}
	return false
}

// filtersSubsume reports whether a route conditioned by a admits everything one
// conditioned by b does, which holds when a's conditions are a subset of b's.
func filtersSubsume(a, b []ElementFilter) bool {
	for _, f := range a {
		if !hasFilter(b, f) {
			return false
		}
	}
	return true
}

// reexportGated is reexport, additionally recording the element-filter
// conditions the routes that surfaced the name impose on it. Routes accumulate:
// a name is a member of the importing namespace when any one of them admits it,
// so an unfiltered import re-exports it whatever another route filters out.
func (idx *Index) reexportGated(fqn string, sym *Symbol, doc string, private bool, gates [][]ElementFilter) {
	claim := idx.reexport(fqn, sym, doc, private)
	if claim == nil {
		return // the namespace declares it; nothing was borrowed
	}
	widened := false
	for _, gate := range gates {
		widened = claim.record(gateRoute{private: private, filters: gate}) || widened
	}
	// A namespace importing this one onward copied the narrower routes, so a
	// widened claim has to reach it too (see routesOnward).
	if parent, _ := splitFQN(fqn); widened && parent != "" {
		idx.markGained(parent)
	}
}

// record adds a route to the claim, unless one of the same visibility already
// admits at least as much — an unconditional route makes the conditional ones
// beside it redundant, and re-expanding an importer records nothing new. Keeping
// only the routes no other subsumes also bounds the set, which is what lets a
// cycle of filtered imports settle. It reports whether the claim now admits more
// than it did.
func (c *reexportClaim) record(route gateRoute) bool {
	for _, have := range c.routes {
		if have.private == route.private && filtersSubsume(have.filters, route.filters) {
			return false
		}
	}
	kept := c.routes[:0]
	for _, have := range c.routes {
		if have.private != route.private || !filtersSubsume(route.filters, have.filters) {
			kept = append(kept, have)
		}
	}
	c.routes = append(kept, route)
	return true
}

// ReexportGates returns the element-filter conditions the name fqn is subject to
// when it reaches a lookup as sym, one entry per route that surfaced it: the
// conditions of the import that surfaced it along that route plus the importing
// namespace's own `filter` members. A name a namespace declares itself has no
// route and so no conditions.
//
// The resolver reads them and evaluates them against its semantic model, and
// admits the element when any route's conditions all hold: a candidate every
// route rejects is not a member of the namespace it appears under, so no route
// to it resolves (KerML 8.2.4).
// A private import's route only answers a lookup made from within the importing
// namespace, which from names ("" for one made from anywhere else).
func (idx *Index) ReexportGates(doc, fqn string, sym *Symbol, from string) [][]ElementFilter {
	claims := idx.reexportDocs.at(reexportKey{fqn: fqn, sym: sym})
	parent, _ := splitFQN(fqn)
	if parent == "" {
		// Each document owns its root namespace alone, so only the routes of the
		// document looking the name up gate it — and they are its own.
		return claims[doc].gateRoutes(true)
	}
	inside := withinNamespace(from, parent)
	var out [][]ElementFilter
	for _, claim := range claims {
		out = append(out, claim.gateRoutes(inside)...)
	}
	return out
}

// ReexportVisible reports whether a lookup made in doc reaches sym under the
// name fqn. A root-level name a wildcard import surfaced is a member of the
// importing document's own root namespace, so it is not visible in another
// document (KerML 8.2.3.3); a name under a namespace is visible wherever that
// namespace is.
func (idx *Index) ReexportVisible(doc, fqn string, sym *Symbol) bool {
	if parent, _ := splitFQN(fqn); parent != "" {
		return true
	}
	if !idx.reexported.at(fqn).has(sym) {
		return true // declared under this name rather than borrowed
	}
	return idx.reexportDocs.at(reexportKey{fqn: fqn, sym: sym})[doc] != nil
}

// gateRoutes returns the conditions of the routes a claim recorded that a lookup
// may take, and none for an absent claim.
func (c *reexportClaim) gateRoutes(private bool) [][]ElementFilter {
	if c == nil {
		return nil
	}
	out := make([][]ElementFilter, 0, len(c.routes))
	for _, route := range c.routes {
		if route.private && !private {
			continue
		}
		out = append(out, route.filters)
	}
	return out
}

// nonZeroFilters drops the absent conditions of an unfiltered import.
func nonZeroFilters(filters []ElementFilter) []ElementFilter {
	var out []ElementFilter
	for _, f := range filters {
		if !f.IsZero() {
			out = append(out, f)
		}
	}
	return out
}

// reexport registers sym under fqn on doc's behalf and returns doc's writable
// claim on it, or nil when the namespace declares sym there itself.
func (idx *Index) reexport(fqn string, sym *Symbol, doc string, private bool) *reexportClaim {
	if idx.hasFQN(fqn, sym) {
		if !idx.reexported.at(fqn).has(sym) {
			return nil // declared here, not borrowed
		}
	} else {
		idx.link(fqn, sym) // claimReexport notes the gain
	}
	return idx.claimReexport(reexportKey{fqn: fqn, sym: sym}, doc, !private)
}

// claimReexport records that doc's wildcard import surfaces the re-export key,
// publicly or not, and updates the marks a lookup reads. A name is exported when
// any import that surfaced it was public, so a public claim clears the hidden
// mark a private one left. It returns doc's writable claim.
func (idx *Index) claimReexport(key reexportKey, doc string, public bool) *reexportClaim {
	docs := idx.writableClaims(key)
	claim, claimed := docs[doc]
	if claimed && (claim.public || !public) {
		return claim // nothing new
	}
	if !claimed {
		claim = &reexportClaim{}
		docs[doc] = claim
	}
	claim.public = claim.public || public
	writableMap(idx.docReexports, doc)[key] = true
	idx.applyReexportMarks(key, docs)
	parent, _ := splitFQN(key.fqn)
	idx.markGained(parent) // a public claim can un-hide it, which exports it onward
	return claim
}

// dropClaim forgets doc's claim on a re-export, deregistering the name once no
// document surfaces it any more and re-hiding it when only private imports
// remain.
func (idx *Index) dropClaim(key reexportKey, doc string) {
	if _, claimed := idx.reexportDocs.at(key)[doc]; !claimed {
		return
	}
	docs := idx.writableClaims(key)
	delete(docs, doc) // the routes this document recorded go with its claim
	if len(docs) == 0 {
		idx.reexportDocs.del(key)
		idx.deregister(key.fqn, key.sym)
		return
	}
	idx.applyReexportMarks(key, docs)
	parent, _ := splitFQN(key.fqn)
	idx.markLost(parent) // only private imports may remain, hiding it again
}

// purgeReexport drops a re-export outright, along with every document's claim
// on it.
func (idx *Index) purgeReexport(key reexportKey) {
	for doc := range idx.reexportDocs.at(key) {
		claimed := writableMap(idx.docReexports, doc)
		delete(claimed, key)
		if len(claimed) == 0 {
			idx.docReexports.del(doc)
		}
	}
	idx.reexportDocs.del(key)
	idx.deregister(key.fqn, key.sym)
}

// writableClaims returns the claims on key that this index may write to. A claim
// the frozen base recorded is copied with them: recording a route on it would
// otherwise change what every index over that base re-exports.
func (idx *Index) writableClaims(key reexportKey) map[string]*reexportClaim {
	if docs, owned := idx.reexportDocs.own[key]; owned {
		idx.reexportDocs.gen.bump()
		return docs
	}
	shared, _ := idx.reexportDocs.below(key)
	docs := make(map[string]*reexportClaim, len(shared)+1)
	for doc, claim := range shared {
		copied := *claim
		copied.routes = append([]gateRoute(nil), claim.routes...)
		docs[doc] = &copied
	}
	idx.reexportDocs.set(key, docs)
	return docs
}

// applyReexportMarks brings the reexported and hidden marks in line with docs,
// the claims on key: a claimed name is re-exported, and hidden while every
// document that surfaced it did so with a private import (KerML 8.2.3.3).
func (idx *Index) applyReexportMarks(key reexportKey, docs map[string]*reexportClaim) {
	if len(docs) == 0 {
		clearMark(idx.reexported, key.fqn, key.sym)
		clearMark(idx.hidden, key.fqn, key.sym)
		return
	}
	setMark(idx.reexported, key.fqn, key.sym)
	for _, claim := range docs {
		if claim.public {
			clearMark(idx.hidden, key.fqn, key.sym)
			return
		}
	}
	setMark(idx.hidden, key.fqn, key.sym)
}

// symbolSet is the few symbols marked under one name; a linear scan of it beats
// a map for the one or two entries it holds.
type symbolSet []*Symbol

func (s symbolSet) has(sym *Symbol) bool {
	return slices.Contains(s, sym)
}

func setMark(marks *layer[string, symbolSet], fqn string, sym *Symbol) {
	if owned, ok := marks.own[fqn]; ok {
		if !owned.has(sym) {
			marks.gen.bump()
			marks.own[fqn] = append(owned, sym)
		}
		return
	}
	shared, _ := marks.below(fqn)
	if shared.has(sym) {
		return
	}
	out := make(symbolSet, len(shared), len(shared)+1)
	copy(out, shared)
	marks.set(fqn, append(out, sym))
}

func clearMark(marks *layer[string, symbolSet], fqn string, sym *Symbol) {
	i := slices.Index(marks.at(fqn), sym)
	if i < 0 {
		return
	}
	at := slices.Delete(writableSlice(marks, fqn), i, i+1)
	if len(at) == 0 {
		marks.del(fqn)
		return
	}
	marks.set(fqn, at)
}

// indexScope walks a scope, recording each distinct symbol under its FQN and
// recursing into child scopes. prefix is the FQN of the owning scope ("" at
// the document root). Every recorded (fqn, symbol) pair is also tracked as a
// contribution of the named document.
func (idx *Index) indexScope(doc string, scope *Scope, prefix string) {
	seen := make(map[*Symbol]bool)
	for _, sym := range scope.syms {
		if seen[sym] {
			continue // symbol registered under both short and primary key
		}
		seen[sym] = true

		// Index under primary FQN
		fqn := joinFQN(prefix, sym.Name)
		idx.register(fqn, sym)
		idx.declaredAt.set(sym, fqn)
		idx.addContribution(doc, fqnEntry{fqn: fqn, sym: sym})

		// Also index under short name FQN if different
		// Try cached shortName first (for stdlib), fallback to extracting from Decl
		shortName := sym.ShortName
		if shortName == "" {
			id, _ := DeclIdent(sym.Decl)
			shortName = id.ShortName
		}
		if shortName != "" && shortName != sym.Name {
			shortFQN := joinFQN(prefix, shortName)
			idx.register(shortFQN, sym)
			idx.addContribution(doc, fqnEntry{fqn: shortFQN, sym: sym})
		}

		// Extract wildcard imports and filters from packages/namespaces
		if sym.Kind == SymbolPackage || sym.Kind == SymbolNamespace {
			if wildcards := extractWildcardImports(sym.Decl, sym.Scope); len(wildcards) > 0 {
				idx.setWildcardImports(fqn, doc, wildcards)
			}
			idx.SetNamespaceFilters(fqn, doc, extractNamespaceFilters(sym.Decl, sym.Scope))
		}

		if sym.Scope != nil {
			idx.indexScope(doc, sym.Scope, fqn)
		}
	}
}

// addContribution records that doc registered a symbol under a name.
func (idx *Index) addContribution(doc string, e fqnEntry) {
	idx.contributions.set(doc, append(writableSlice(idx.contributions, doc), e))
}

// joinFQN joins a prefix and a name with "::".
func joinFQN(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "::" + name
}

// extractWildcardImports extracts the wildcard imports of a Package, Namespace,
// or RootNamespace AST node: the raw qualified name text (e.g. "ISQBase") and
// declared visibility of each `import <name>::*` statement.
func extractWildcardImports(decl ast.Node, scope *Scope) []WildcardImport {
	var out []WildcardImport
	for _, m := range namespaceMembers(decl) {
		imp, ok := m.(*ast.Import)
		if !ok || imp.Kind != ast.ImportNamespace || imp.Imported == nil {
			continue
		}
		wi := WildcardImport{
			Target:  qualifiedNameText(imp.Imported),
			Private: imp.Visibility == ast.VisibilityPrivate,
		}
		if imp.FilterExpr != nil {
			wi.Filter = ElementFilter{Expr: imp.FilterExpr, Scope: scope, Span: imp.FilterExpr.Span()}
		}
		out = append(out, wi)
	}
	return out
}

// qualifiedNameText renders a QualifiedName as "A::B::C".
func qualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	var parts []string
	for _, seg := range qn.Parts {
		parts = append(parts, seg.Text)
	}
	return joinQualifiedName(parts)
}

// joinQualifiedName joins parts with "::".
func joinQualifiedName(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "::"
		}
		result += part
	}
	return result
}

// LookupQualified returns the symbols a qualified reference from outside the
// naming namespace reaches under the exact fully-qualified name. A namespace's
// own member shadows one of the same name that a wildcard import re-exported
// through it, as in SI::min, which is SI's minute and not the imported min
// function, and a name only a *private* import surfaced is not reachable at all:
// it is a member of the namespace, but not a visible one (KerML 8.2.3.3).
func (idx *Index) LookupQualified(fqn string) []*Symbol {
	return idx.LookupQualifiedFrom(fqn, "")
}

// LookupQualifiedFrom is LookupQualified as seen from the namespace named by
// fromFQN. A private import is visible inside the namespace that declares it and
// inside everything nested in it, so a reference made from there — including the
// target of an alias the namespace declares — still reaches a privately imported
// name that the same lookup from anywhere else does not (KerML 8.2.3.3).
//
// fromFQN is the FQN of the referring namespace; "" means "from outside", which
// is what an ordinary qualified reference elsewhere in the workspace gets.
func (idx *Index) LookupQualifiedFrom(fqn, fromFQN string) []*Symbol {
	syms := idx.fqn.at(fqn)
	imported := idx.reexported.at(fqn)
	if len(imported) == 0 {
		return syms
	}
	hidden := idx.hidden.at(fqn)
	// A root-level name belongs to the importing document's own root namespace, so
	// a private import of it is answered per document by ReexportVisible, not here.
	if len(hidden) > 0 && namespaceOf(fqn) != "" && !withinNamespace(fromFQN, namespaceOf(fqn)) {
		visible := make([]*Symbol, 0, len(syms))
		for _, sym := range syms {
			if !hidden.has(sym) {
				visible = append(visible, sym)
			}
		}
		syms = visible
	}
	owned := make([]*Symbol, 0, len(syms))
	for _, sym := range syms {
		if !imported.has(sym) {
			owned = append(owned, sym)
		}
	}
	if len(owned) == 0 {
		return syms
	}
	return owned
}

// Declaring returns the symbol fqn declares, and nil when fqn only re-exports a
// declaration made elsewhere. The lookup is made from fqn itself, so a private
// member is visible where it is declared.
func (idx *Index) Declaring(fqn string) *Symbol {
	for _, sym := range idx.LookupQualifiedFrom(fqn, fqn) {
		if sym != nil && HasFQN(sym, fqn) {
			return sym
		}
	}
	return nil
}

// HiddenFrom reports whether every symbol registered under fqn is one only a
// private import surfaced there, seen from the namespace fromFQN. It is the
// reason LookupQualifiedFrom found nothing, so a caller that falls back to
// another lookup route — the qualified walk's inheritance-aware member search,
// which reaches cached symbols through LookupDirectChildren — asks here first
// and stops, rather than resurfacing a name KerML 8.2.3.3 hides.
func (idx *Index) HiddenFrom(fqn, fromFQN string) bool {
	hidden := idx.hidden.at(fqn)
	if len(hidden) == 0 || withinNamespace(fromFQN, namespaceOf(fqn)) {
		return false
	}
	for _, sym := range idx.fqn.at(fqn) {
		if !hidden.has(sym) {
			return false
		}
	}
	return true
}

// namespaceOf returns the FQN of the namespace a qualified name names a member
// of: "A::B::C" -> "A::B", and "" for a top-level name.
func namespaceOf(fqn string) string {
	i := lastSeparator(fqn)
	if i < 0 {
		return ""
	}
	return fqn[:i]
}

// withinNamespace reports whether a reference made from the namespace fromFQN
// sees ns's private memberships, which it does when it *is* ns or is nested
// inside it. A reference from outside any namespace ("") never does, and neither
// does one from a namespace that merely shares a name prefix ("A::BC" is not in
// "A::B").
func withinNamespace(fromFQN, ns string) bool {
	if fromFQN == "" {
		return false
	}
	if ns == "" {
		return false
	}
	if fromFQN == ns {
		return true
	}
	return len(fromFQN) > len(ns)+2 && fromFQN[:len(ns)] == ns && fromFQN[len(ns):len(ns)+2] == "::"
}

// FQNs returns every fully-qualified name registered in the index, sorted.
func (idx *Index) FQNs() []string {
	out := idx.fqn.keys()
	sort.Strings(out)
	return out
}

// FQNsEndingIn returns up to limit registered fully-qualified names whose last
// segment is name, in name order. Used to suggest a candidate for a reference
// whose qualifying namespace is not loaded.
func (idx *Index) FQNsEndingIn(name string, limit int) []string {
	if name == "" || limit <= 0 {
		return nil
	}
	out := idx.bySegment.at(name)
	if len(out) > limit {
		out = out[:limit]
	}
	return slices.Clone(out)
}

// WildcardImportsOf returns the wildcard-import targets recorded for the
// namespace registered under fqn ("" for a document root), over the documents
// declaring it in name order.
func (idx *Index) WildcardImportsOf(fqn string) []WildcardImport {
	byDoc := idx.wildcardMeta.at(fqn)
	if len(byDoc) == 0 {
		return nil
	}
	docs := make([]string, 0, len(byDoc))
	for doc := range byDoc {
		docs = append(docs, doc)
	}
	sort.Strings(docs)
	var out []WildcardImport
	for _, doc := range docs {
		out = append(out, byDoc[doc]...)
	}
	return out
}

// exportedChildren returns the direct children of prefix that a wildcard import
// of prefix surfaces: everything but what prefix's own private imports brought
// in, which stays visible only inside prefix (KerML 8.2.3.3).
func (idx *Index) exportedChildren(prefix string) []*Symbol {
	return idx.LookupDirectChildrenFrom(prefix, "")
}

// childKeys returns the keys registered directly under prefix, in name order.
// The copy lets callers deregister while they iterate.
func (idx *Index) childKeys(prefix string) []string {
	kids := idx.children.at(prefix)
	if len(kids) == 0 {
		return nil
	}
	return slices.Clone(kids)
}

// LookupDirectChildren returns all symbols whose FQN is exactly prefix::name
// (direct children of the given prefix). This supports wildcard imports from
// packages that don't have populated Scopes.
func (idx *Index) LookupDirectChildren(prefix string) []*Symbol {
	if prefix == "" {
		return nil // a document root's members are reached through its scope
	}
	return idx.lookupDirectChildren(directChildrenKey{
		prefix:        prefix,
		allowsPrivate: true,
	})
}

func (idx *Index) lookupDirectChildren(key directChildrenKey) []*Symbol {
	generation := idx.generation.get()
	idx.directChildrenMu.Lock()
	idx.resetDirectChildrenCachesLocked(generation)
	if out, ok := idx.directChildrenCache[key]; ok {
		idx.directChildrenMu.Unlock()
		return out
	}
	idx.directChildrenMu.Unlock()

	var out []*Symbol
	keys := idx.childKeys(key.prefix)
	seen := make(map[*Symbol]bool, len(keys))
	for _, fqn := range keys {
		hidden := idx.hidden.at(fqn)
		for _, sym := range idx.fqn.at(fqn) {
			if seen[sym] || (!key.allowsPrivate && hidden.has(sym)) {
				continue
			}
			seen[sym] = true
			out = append(out, sym)
		}
	}
	idx.directChildrenMu.Lock()
	if idx.generation.get() == generation {
		idx.directChildrenCache[key] = out
	}
	idx.directChildrenMu.Unlock()
	return out
}

// resetDirectChildrenCachesLocked drops both direct-children caches when the
// index has changed since they were filled. Callers hold directChildrenMu.
func (idx *Index) resetDirectChildrenCachesLocked(generation uint64) {
	if idx.directChildrenGeneration != generation {
		idx.directChildrenCache = make(map[directChildrenKey][]*Symbol)
		idx.directChildrenByName = make(map[directChildrenKey]map[string][]*Symbol)
		idx.directChildrenGeneration = generation
	}
}

// LookupDirectChildrenNamed returns the direct children of prefix whose leaf
// name or short name is name, in LookupDirectChildren order, without a scan.
func (idx *Index) LookupDirectChildrenNamed(prefix, name string) []*Symbol {
	if prefix == "" || name == "" {
		return nil
	}
	return idx.lookupDirectChildrenNamed(directChildrenKey{
		prefix:        prefix,
		allowsPrivate: true,
	}, name)
}

// LookupDirectChildrenNamedFrom is LookupDirectChildrenNamed with the visibility
// of LookupDirectChildrenFrom, as seen from fromFQN ("" meaning from outside).
func (idx *Index) LookupDirectChildrenNamedFrom(prefix, fromFQN, name string) []*Symbol {
	if prefix == "" || name == "" {
		return nil
	}
	return idx.lookupDirectChildrenNamed(directChildrenKey{
		prefix:        prefix,
		allowsPrivate: withinNamespace(fromFQN, prefix),
	}, name)
}

func (idx *Index) lookupDirectChildrenNamed(key directChildrenKey, name string) []*Symbol {
	generation := idx.generation.get()
	idx.directChildrenMu.Lock()
	idx.resetDirectChildrenCachesLocked(generation)
	byName, ok := idx.directChildrenByName[key]
	idx.directChildrenMu.Unlock()
	if ok {
		return byName[name]
	}

	children := idx.lookupDirectChildren(key)
	byName = make(map[string][]*Symbol, len(children))
	for _, sym := range children {
		leaf := lastSegment(sym.Name)
		byName[leaf] = append(byName[leaf], sym)
		if sym.ShortName != "" && sym.ShortName != leaf {
			byName[sym.ShortName] = append(byName[sym.ShortName], sym)
		}
	}
	idx.directChildrenMu.Lock()
	if idx.generation.get() == generation && idx.directChildrenGeneration == generation {
		idx.directChildrenByName[key] = byName
	}
	idx.directChildrenMu.Unlock()
	return byName[name]
}

// lastSegment returns the last "::"-separated segment of a possibly qualified name.
func lastSegment(name string) string {
	if i := lastSeparator(name); i >= 0 {
		return name[i+2:]
	}
	return name
}

// RootBinding is a name registered at the index root and the symbol it names.
type RootBinding struct {
	Name string
	Sym  *Symbol
}

// TopLevelBindings returns the names registered at the root of the index as seen
// from doc ("" meaning from outside every document) and the symbol each names:
// the library's top-level packages and every document's top-level declarations,
// less the names only another document's private import surfaced (KerML
// 8.2.3.3). A caller gating those names by their element filters needs the name,
// since a borrowed symbol's own name is not the root name it appears under.
func (idx *Index) TopLevelBindings(doc string) []RootBinding {
	claimed := idx.docReexports.at(doc)
	var out []RootBinding
	seen := make(map[*Symbol]bool)
	for _, fqn := range idx.childKeys("") {
		hidden := idx.hidden.at(fqn)
		for _, sym := range idx.fqn.at(fqn) {
			if seen[sym] {
				continue
			}
			if hidden.has(sym) && !claimed[reexportKey{fqn: fqn, sym: sym}] {
				continue // only some other document's private import surfaced it
			}
			seen[sym] = true
			out = append(out, RootBinding{Name: fqn, Sym: sym})
		}
	}
	return out
}

// LookupDirectChildrenFrom is LookupDirectChildren as seen from the namespace
// named by fromFQN ("" meaning from outside): children that only prefix's own
// private imports brought in are dropped (KerML 8.2.3.3).
func (idx *Index) LookupDirectChildrenFrom(prefix, fromFQN string) []*Symbol {
	if prefix == "" {
		return nil // a document root's members are reached through its scope
	}
	return idx.lookupDirectChildren(directChildrenKey{
		prefix:        prefix,
		allowsPrivate: withinNamespace(fromFQN, prefix),
	})
}

// GetFQN returns the fully-qualified name for a symbol by walking its owner scope chain.
// Returns the local name if the symbol has no owner scope (root-level symbol).
func (idx *Index) GetFQN(sym *Symbol) string {
	return FQNOf(sym)
}

// FQNOf returns a symbol's fully-qualified name from its owner scope chain, so
// a caller holding a symbol but no index can still name it. An unnamed owner
// contributes no segment: a part nested in an anonymous part of Mid is Mid::inner.
func FQNOf(sym *Symbol) string {
	if sym == nil {
		return ""
	}

	// Collect the scope chain from the symbol up to the root, leaf first. A
	// nesting deeper than the array is rare, so it grows on the heap only then.
	var buf [16]string
	parts := buf[:0]
	parts = append(parts, sym.Name)
	size := len(sym.Name)
	scope := sym.OwnerScope
	for scope != nil && scope.Owner() != nil {
		owner := scope.Owner()
		if owner.Name != "" {
			parts = append(parts, owner.Name)
			size += len(owner.Name) + len(fqnSeparator)
		}
		scope = owner.OwnerScope
	}

	if len(parts) == 1 {
		return parts[0]
	}
	var out strings.Builder
	out.Grow(size)
	for i := len(parts) - 1; i >= 0; i-- {
		if i != len(parts)-1 {
			out.WriteString(fqnSeparator)
		}
		out.WriteString(parts[i])
	}
	return out.String()
}

// HasFQN reports whether sym's fully-qualified name is fqn, without building
// that name: the segments are compared against the owner chain from the end,
// skipping unnamed owners as FQNOf does.
func HasFQN(sym *Symbol, fqn string) bool {
	if sym == nil {
		return fqn == ""
	}
	rest, name, scope := fqn, sym.Name, sym.OwnerScope
	for {
		if !strings.HasSuffix(rest, name) {
			return false
		}
		rest = rest[:len(rest)-len(name)]
		owner := namedOwner(scope)
		if owner == nil {
			return rest == ""
		}
		if !strings.HasSuffix(rest, fqnSeparator) {
			return false
		}
		rest = rest[:len(rest)-len(fqnSeparator)]
		name, scope = owner.Name, owner.OwnerScope
	}
}

// namedOwner returns the nearest named symbol owning scope or an ancestor of
// it, or nil at the document root.
func namedOwner(scope *Scope) *Symbol {
	for scope != nil && scope.Owner() != nil {
		if owner := scope.Owner(); owner.Name != "" {
			return owner
		}
		scope = scope.Owner().OwnerScope
	}
	return nil
}

// DocumentOfRoot returns the name of the document whose root scope this is, or
// "" for any other scope.
func (idx *Index) DocumentOfRoot(scope *Scope) string {
	return idx.docOfRoot.at(scope)
}

// DocumentRoot returns the root scope for the named document, or nil.
func (idx *Index) DocumentRoot(name string) *Scope {
	return idx.docRoots.at(name)
}

// Documents returns the names of every document with a root scope, bundled
// library content included, sorted for deterministic iteration.
func (idx *Index) Documents() []string {
	out := append([]string(nil), idx.docRoots.keys()...)
	sort.Strings(out)
	return out
}

// WorkspaceDocuments returns the names of the documents holding workspace
// content — every document with a root scope that is not marked as bundled
// library content — sorted for deterministic iteration.
func (idx *Index) WorkspaceDocuments() []string {
	var out []string
	for _, name := range idx.docRoots.keys() {
		if !idx.libraryDocs.at(name).Tier.Library() {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// DocumentKind returns a document's recorded language, or infers it from its
// name when the document was added without an explicit language.
func (idx *Index) DocumentKind(name string) source.Kind {
	if kind, ok := idx.docKinds.get(name); ok {
		return kind
	}
	return source.KindOf(name)
}

// NewIndexFromDoc builds an Index containing a single document.
func NewIndexFromDoc(name string, root *ast.RootNamespace) *Index {
	idx := NewIndex()
	idx.AddDocument(name, root)
	return idx
}
