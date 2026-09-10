package resolve

import (
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// MemberLookup interface abstracts semantic model for inheritance-aware member resolution.
// Implemented by *semantics.Model.
type MemberLookup interface {
	// LookupMember searches for a member by name in sym's scope and inherited scopes.
	LookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool)
	// LookupContributedMember searches the inherited scopes only, skipping
	// what sym itself declares.
	LookupContributedMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool)
}

// supertypeProvider is the part of the semantic model that reports the features
// a declaration specializes, including the ones it redefines implicitly. A
// nameless parameter takes its name from the parameter it redefines, which only
// the model can match (KerML 7.3.4.5). *semantics.Model implements it.
type supertypeProvider interface {
	DirectSupertypes(sym *symbols.Symbol) []*symbols.Symbol
}

// maskChecker is the part of the semantic model that reports redefinition
// masking: which of a type's inheritable members it does not inherit because
// one of its features redefines them. *semantics.Model implements it.
type maskChecker interface {
	InheritanceMasked(sym, candidate *symbols.Symbol) bool
	InheritanceMaskedRedefining(sym, candidate *symbols.Symbol) bool
	NamingRedefiner(sym, masked *symbols.Symbol) *symbols.Symbol
	NamingRedefinerRedefining(sym, masked *symbols.Symbol) *symbols.Symbol
}

// elementFilterChecker is the part of the semantic model that decides an element
// filter: whether the element an import would surface is selected by the
// condition restricting it (KerML 8.2.4). A condition classifies a candidate by
// the metadata annotating it, which only the model knows.
// *semantics.Model implements it.
type elementFilterChecker interface {
	SatisfiesElementFilter(f symbols.ElementFilter, cand *symbols.Symbol) bool
}

// resolution is a memoized lookup outcome.
type resolution struct {
	sym *symbols.Symbol
	ok  bool
}

type featureChainKey struct {
	scope *symbols.Scope
	node  *ast.FeatureChainExpr
}

// modeMemoKey keys a lookup made in a non-default resolution mode: a filter
// condition's own names bypass the namespace's filters, and an `import all` or
// expose target reaches every membership. Such an answer is memoized apart from
// the ordinary one, which must never inherit it.
type modeMemoKey struct {
	at          ast.Node
	condition   bool
	allVisible  bool
	hide        ast.Node
	prefix      bool
	borrowedOut bool
}

type filteredMemoKey struct {
	qn               *ast.QualifiedName
	decl             ast.Node
	prefix           bool
	skipBorrowedName bool
}

// Resolver performs lazy name resolution over a symbol index, memoizing results
// keyed by the reference AST node and collecting diagnostics.
type Resolver struct {
	idx      *symbols.Index
	memo     map[ast.Node]resolution
	modeMemo map[modeMemoKey]resolution
	filtered map[filteredMemoKey]resolution
	// resolving holds the depth (see enter) of each lookup on the stack, so a
	// re-entrant query of the same name fails instead of recursing.
	resolving map[ast.Node]int
	// frames holds, per lookup on the stack, the shallowest depth a guard has
	// cut a query short for since the lookup began (see leave).
	frames []int
	// featureChains are resolved per scope because a chain's leading operand
	// can resolve differently in different document scopes.
	featureChains map[featureChainKey]resolution
	parts         map[*ast.QualifiedName][]*symbols.Symbol
	// aliasNames are the alias memberships a segment's name went through, kept
	// beside parts: the segment reaches the aliased element, and a consumer that
	// asks about the name written (a rename) needs the alias too.
	aliasNames map[*ast.QualifiedName][]*symbols.Symbol
	// endpoints are the vertices transition endpoints resolve to, memoized per
	// name node: lowering consumes what this tier resolved (see ResolveEndpoint).
	endpoints map[*ast.QualifiedName]resolution
	// initials are the body members `first x` names, memoized per node: a name
	// no member of the body answers to is a label the node declares.
	initials map[*ast.InitialNode]*symbols.Symbol
	// imports are the import declarations of a namespace-bearing node, found once
	// and kept: see (*Resolver).importsOf.
	imports          map[ast.Node][]*ast.Import
	importStack      map[*ast.Import]bool
	resolvingImports map[*ast.Import]bool
	Diagnostics      []Diagnostic
	// quiet is nonzero while a lookup is made on behalf of a semantic query
	// rather than a reference in the document being resolved.
	quiet int
	// inCondition is nonzero while a filter condition's own names are resolved,
	// which the condition does not filter.
	inCondition int
	// allVisible is nonzero while the target of an `import all` (or of an
	// expose) is resolved, which reaches every membership, not the visible ones.
	allVisible int
	// trial is the name a ProbeReading is reading, whose segment records a
	// failed probe keeps: the reader wants what each segment reached.
	trial *ast.QualifiedName
	// nsFilters are the `filter` members of a namespace, extracted once per scope.
	nsFilters map[*symbols.Scope][]symbols.ElementFilter
	// viewFilters memoizes the conditions a view inherits from the views it
	// specializes; viewFiltersInProgress cuts the re-entrant query short.
	viewFilters           map[*symbols.Scope][]symbols.ElementFilter
	viewFiltersInProgress map[*symbols.Scope]bool
	// payloads are the accept-node payloads a scope's body shares, collected
	// once per scope: see (*Resolver).acceptPayload.
	payloads map[*symbols.Scope]map[string]*symbols.Symbol
	// redefined memoizes the features a declaration redefines, explicitly or as
	// an end: see (*Resolver).redefinedFeatures.
	redefined map[*symbols.Symbol][]*symbols.Symbol
	// bodyOwners memoizes the metadata definition owning an annotation body
	// scope the document pass has not stamped: see (*Resolver).scopeOwner.
	bodyOwners map[*symbols.Scope]*symbols.Symbol
	// effNames memoizes whether a feature named by a redefinition binds that
	// name: see (*Resolver).BindsName.
	effNames map[*symbols.Symbol]bool
	model    MemberLookup             // Optional *semantics.Model for inheritance-aware member lookup
	naming   map[*symbols.Symbol]bool // effective names being computed, for cycle detection
	// inheritedImports are the declarations whose supertypes' imports are being
	// searched, so a specialization cycle ends the walk.
	inheritedImports map[*symbols.Symbol]bool
	// constraintRefs are the requirements referenced by the require/assume
	// members whose bodies are being walked, innermost last: such a body may
	// redefine a feature of the requirement it references by plain name.
	constraintRefs []constraintRef
	// aliasTargets memoizes the canonical target of each alias. A separate
	// resolving set makes cycles fail without recursing indefinitely.
	aliasTargets   map[*symbols.Symbol]resolution
	resolvingAlias map[*symbols.Symbol]bool
	// suggestions are the spellings an unresolvable name may have meant, kept
	// per name and scope; names is the index's name table they are looked up in.
	// suggesting holds the suggestions being scored, so scoring one cannot
	// recurse into scoring itself.
	suggestions map[suggestKey]suggestion
	names       *suggest.Table
	suggesting  map[suggestKey]bool
	// probing holds the member hints being computed, for the same reason.
	probing map[memberKey]bool
	// document is the document ResolveDocument is resolving, so a reference
	// reached in another one is not reported against it (see foreignScope).
	document          string
	reportedQualified map[*ast.QualifiedName]bool
	// ambiguities are the candidate counts of the qualified names that failed
	// by naming several elements, so a consumer can tell that apart from none.
	ambiguities map[*ast.QualifiedName]int
	// readings are the answers to qualified names read in a chosen scope, kept
	// apart from the written name's memo: see ReadQualified.
	readings map[readingKey]Reading
	// valuesInProgress are the usages whose value expression is being read for
	// the type it names, so a value naming its own feature ends the walk.
	valuesInProgress map[*ast.Usage]bool
	// invocationNames are the names invocations call, whose last segment may
	// denote several declarations: see ResolveInvocationName.
	invocationNames map[*ast.QualifiedName]bool
	// scratch are the Scratch calls in progress, innermost last.
	scratch []*scratchFrame
}

// scratchFrame is one Scratch call: the nodes it disowns and the entries
// first memoized under it.
type scratchFrame struct {
	transient map[ast.Node]bool
	journal   []journaled
}

type journaled struct {
	node ast.Node
	drop func()
}

// Scratch runs f, then forgets what f memoized about the transient nodes, so
// syntax the model does not own (a request expression) is not retained.
func (r *Resolver) Scratch(transient map[ast.Node]bool, f func()) {
	frame := &scratchFrame{transient: transient}
	r.scratch = append(r.scratch, frame)
	defer func() {
		r.scratch = r.scratch[:len(r.scratch)-1]
		var parent *scratchFrame
		if n := len(r.scratch); n > 0 {
			parent = r.scratch[n-1]
		}
		for _, j := range frame.journal {
			switch {
			case frame.transient[j.node]:
				j.drop()
			case parent != nil:
				parent.journal = append(parent.journal, j)
			}
		}
	}()
	f()
}

// MemoSize is the number of resolutions this resolver retains.
func (r *Resolver) MemoSize() int {
	return len(r.memo) + len(r.modeMemo) + len(r.filtered) + len(r.featureChains) +
		len(r.parts) + len(r.aliasNames) + len(r.endpoints) + len(r.readings) +
		len(r.invocationNames) + len(r.ambiguities) + len(r.reportedQualified) +
		len(r.initials) + len(r.imports) + len(r.suggestions)
}

// journalNew lets the enclosing Scratch drop m[k], about to be written for
// node the first time.
func journalNew[K comparable, V any](r *Resolver, m map[K]V, k K, node ast.Node) {
	if len(r.scratch) == 0 {
		return
	}
	if _, had := m[k]; had {
		return
	}
	r.Journal(node, func() { delete(m, k) })
}

// Journal registers drop to run when the enclosing Scratch, if any, ends with
// node transient: how a side table keyed by node joins the resolver's lifecycle.
func (r *Resolver) Journal(node ast.Node, drop func()) {
	if r == nil {
		return
	}
	if n := len(r.scratch); n > 0 {
		frame := r.scratch[n-1]
		frame.journal = append(frame.journal, journaled{node: node, drop: drop})
	}
}

// New creates a resolver over the given index.
func New(idx *symbols.Index) *Resolver {
	return &Resolver{
		idx:                   idx,
		memo:                  map[ast.Node]resolution{},
		modeMemo:              map[modeMemoKey]resolution{},
		filtered:              map[filteredMemoKey]resolution{},
		resolving:             map[ast.Node]int{},
		featureChains:         map[featureChainKey]resolution{},
		parts:                 map[*ast.QualifiedName][]*symbols.Symbol{},
		aliasNames:            map[*ast.QualifiedName][]*symbols.Symbol{},
		endpoints:             map[*ast.QualifiedName]resolution{},
		initials:              map[*ast.InitialNode]*symbols.Symbol{},
		imports:               map[ast.Node][]*ast.Import{},
		importStack:           map[*ast.Import]bool{},
		resolvingImports:      map[*ast.Import]bool{},
		naming:                map[*symbols.Symbol]bool{},
		valuesInProgress:      map[*ast.Usage]bool{},
		nsFilters:             map[*symbols.Scope][]symbols.ElementFilter{},
		viewFilters:           map[*symbols.Scope][]symbols.ElementFilter{},
		viewFiltersInProgress: map[*symbols.Scope]bool{},
		payloads:              map[*symbols.Scope]map[string]*symbols.Symbol{},
		redefined:             map[*symbols.Symbol][]*symbols.Symbol{},
		bodyOwners:            map[*symbols.Scope]*symbols.Symbol{},
		effNames:              map[*symbols.Symbol]bool{},

		suggestions: map[suggestKey]suggestion{},
		suggesting:  map[suggestKey]bool{},
		probing:     map[memberKey]bool{},

		inheritedImports:  map[*symbols.Symbol]bool{},
		aliasTargets:      map[*symbols.Symbol]resolution{},
		resolvingAlias:    map[*symbols.Symbol]bool{},
		reportedQualified: map[*ast.QualifiedName]bool{},
		invocationNames:   map[*ast.QualifiedName]bool{},
		ambiguities:       map[*ast.QualifiedName]int{},
		readings:          map[readingKey]Reading{},
	}
}

// recordPart stores the symbol a single segment of a qualified name resolves to.
// Per-segment results are kept here rather than on the AST, which stays
// immutable after parsing so that concurrent readers may share it.
func (r *Resolver) recordPart(qn *ast.QualifiedName, i int, sym *symbols.Symbol) {
	r.resolvedPart(qn, i, sym)
}

// resolvedPart records what segment i of qn named and returns the element it
// reaches: a name bound by an alias reaches the aliased element (see
// AliasedElement), and the alias itself is kept in aliasNames.
func (r *Resolver) resolvedPart(qn *ast.QualifiedName, i int, sym *symbols.Symbol) *symbols.Symbol {
	element := r.AliasedElement(sym)
	if qn == nil || i < 0 || i >= len(qn.Parts) {
		return element
	}
	syms, ok := r.parts[qn]
	if !ok {
		syms = make([]*symbols.Symbol, len(qn.Parts))
		journalNew(r, r.parts, qn, qn)
		r.parts[qn] = syms
	}
	syms[i] = element
	if element != sym {
		aliases, ok := r.aliasNames[qn]
		if !ok {
			aliases = make([]*symbols.Symbol, len(qn.Parts))
			journalNew(r, r.aliasNames, qn, qn)
			r.aliasNames[qn] = aliases
		}
		aliases[i] = sym
	}
	return element
}

// PartSymbol returns the symbol the i-th segment of qn resolved to during a
// previous resolution by this resolver. Callers that need intermediate
// resolutions of a qualified name (a hover over `A::B` in `A::B::C`, say) read
// them here.
func (r *Resolver) PartSymbol(qn *ast.QualifiedName, i int) (*symbols.Symbol, bool) {
	syms, ok := r.parts[qn]
	if !ok || i < 0 || i >= len(syms) || syms[i] == nil {
		return nil, false
	}
	return syms[i], true
}

// EndSymbol returns what the edge end qn resolved to: the vertex a transition
// end was judged as, or otherwise its last segment's symbol.
func (r *Resolver) EndSymbol(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	if res, done := r.endpoints[qn]; done {
		return res.sym, res.ok
	}
	return r.PartSymbol(qn, len(qn.Parts)-1)
}

// resolveInitial binds `first x` to the member of the body x names, if any:
// lowering starts the flow there instead of at the node (see lower).
func (r *Resolver) resolveInitial(scope *symbols.Scope, n *ast.InitialNode) {
	if n.Name == "" {
		return
	}
	if sym, ok := memberPastLabels(scope, n.Name); ok {
		journalNew(r, r.initials, n, n)
		r.initials[n] = sym
	}
}

// memberPastLabels is the member of scope that name reaches past the labels
// initial nodes declare; false where only a label bears it.
func memberPastLabels(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	var members []*symbols.Symbol
	for _, sym := range scope.LookupLocalAll(name) {
		if _, label := sym.Decl.(*ast.InitialNode); label {
			continue
		}
		members = append(members, sym)
	}
	if len(members) == 0 {
		return nil, false
	}
	return symbols.PreferDeclared(members)[0], true
}

// InitialSymbol returns the body member the initial node's name resolved to,
// or false where the name is a label the node itself declares.
func (r *Resolver) InitialSymbol(n *ast.InitialNode) (*symbols.Symbol, bool) {
	sym, ok := r.initials[n]
	return sym, ok
}

// PartAlias returns the alias membership the i-th segment of qn was written as,
// where the name it wrote is an alias of the element PartSymbol reports.
func (r *Resolver) PartAlias(qn *ast.QualifiedName, i int) (*symbols.Symbol, bool) {
	aliases, ok := r.aliasNames[qn]
	if !ok || i < 0 || i >= len(aliases) || aliases[i] == nil {
		return nil, false
	}
	return aliases[i], true
}

// PartName returns the symbol whose name the i-th segment of qn wrote: the alias
// membership when it was written as an alias name, else the element it reached.
func (r *Resolver) PartName(qn *ast.QualifiedName, i int) (*symbols.Symbol, bool) {
	if alias, ok := r.PartAlias(qn, i); ok {
		return alias, true
	}
	return r.PartSymbol(qn, i)
}

// Index returns the symbol index this resolver operates over.
func (r *Resolver) Index() *symbols.Index {
	return r.idx
}

// lookupMember resolves name as a member of sym — declared by it or inherited
// from what it specializes or is typed by — when a semantic model is attached.
func (r *Resolver) lookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if r.model == nil || sym == nil {
		return nil, false
	}
	if found, ok := r.lookupMemberOf(sym, name); ok {
		return found, true
	}
	if sym.Scope == nil {
		return nil, false
	}
	for _, imp := range r.importsOf(sym.Scope.Node()) {
		if r.resolvingImports[imp] {
			continue
		}
		if !r.importPrefixAvailable(sym.Scope, imp, name) {
			continue
		}
		if found, ok := r.matchImport(sym.Scope, imp, name); ok {
			return found, true
		}
	}
	return nil, false
}

// lookupMemberOf resolves name as a member sym declares, inherits, or accepts as
// a trigger payload; what its imports surface is not considered.
func (r *Resolver) lookupMemberOf(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if found, ok := r.model.LookupMember(sym, name); ok {
		return found, true
	}
	return r.triggerPayload(sym, name)
}

// lookupContributedMember resolves name as a member sym inherits or
// reference-subsets, ignoring the members sym declares itself.
func (r *Resolver) lookupContributedMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if r.model == nil || sym == nil {
		return nil, false
	}
	return r.model.LookupContributedMember(sym, name)
}

// SetModel attaches a semantic model for inheritance-aware member resolution.
// Must be called before resolving feature chains if inherited members are needed.
func (r *Resolver) SetModel(model MemberLookup) {
	r.model = model
}

// ResolveQualified resolves a qualified-name reference against the given scope.
// scope may be nil to resolve purely from the global index / document root.
// Later tasks implement the walk; this skeleton reports unresolved.
func (r *Resolver) ResolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	return r.resolveQualified(scope, qn, nil)
}

// Enter begins a lookup that memoizes its answer and returns its depth, which
// the guard protecting it passes to CutShort. A nil resolver has no lookups to
// cut short, so its every answer is settled.
func (r *Resolver) Enter() int {
	if r == nil {
		return 1
	}
	r.frames = append(r.frames, math.MaxInt)
	return len(r.frames)
}

// Leave ends the innermost lookup and reports whether its answer is settled: no
// guard cut a query short for a lookup outside it, so it saw every member.
func (r *Resolver) Leave() bool {
	if r == nil {
		return true
	}
	depth := len(r.frames)
	cut := r.frames[depth-1]
	r.frames = r.frames[:depth-1]
	if depth > 1 && cut < r.frames[depth-2] {
		r.frames[depth-2] = cut
	}
	return cut >= depth
}

// CutShort records that a guard cut a query short for the lookup at depth.
func (r *Resolver) CutShort(depth int) {
	if r == nil {
		return
	}
	if top := len(r.frames) - 1; top >= 0 && depth < r.frames[top] {
		r.frames[top] = depth
	}
}

// resolveQualified is ResolveQualified with an optional filter hiding the
// bindings a reference subsetting's own borrowed name contributes.
func (r *Resolver) resolveQualified(scope *symbols.Scope, qn *ast.QualifiedName, hide *refFilter) (*symbols.Symbol, bool) {
	if qn == nil {
		return nil, false
	}
	if r.foreignScope(scope) {
		var sym *symbols.Symbol
		var ok bool
		r.aside(func() { sym, ok = r.resolveQualified(scope, qn, hide) })
		return sym, ok
	}
	cacheMain := hide == nil || hide.skipNamingTarget || hide.skipBorrowedName
	if cacheMain {
		if res, done := r.memo[qn]; done {
			return res.sym, res.ok
		}
	} else if res, done := r.filtered[filteredMemoKey{
		qn: qn, decl: hide.decl, prefix: hide.skipNamingTarget,
		skipBorrowedName: hide.skipBorrowedName,
	}]; done {
		return res.sym, res.ok
	}
	mode, keyed := r.modeKey(qn, hide)
	if keyed {
		if res, done := r.modeMemo[mode]; done {
			return res.sym, res.ok
		}
	}
	if depth := r.resolving[qn]; depth != 0 {
		r.CutShort(depth)
		return nil, false
	}
	r.resolving[qn] = r.Enter()
	// A feature that took its name from qn is never what qn names.
	res := r.walkQualified(scope, qn, hide.hiding(qn))
	delete(r.resolving, qn)
	if !r.Leave() {
		return res.sym, res.ok
	}
	// A failure met during a semantic query is not memoized: the reference it
	// belongs to must still report when its own document is resolved.
	if keyed {
		if res.ok || r.quiet == 0 {
			journalNew(r, r.modeMemo, mode, qn)
			r.modeMemo[mode] = res
		}
	}
	if (res.ok || r.quiet == 0) && r.allVisible == 0 {
		if cacheMain {
			if res.ok || hide == nil {
				r.memoize(qn, res)
			}
		} else if res.ok || r.quiet == 0 {
			key := filteredMemoKey{
				qn: qn, decl: hide.decl, prefix: hide.skipNamingTarget,
				skipBorrowedName: hide.skipBorrowedName,
			}
			journalNew(r, r.filtered, key, qn)
			r.filtered[key] = res
		}
	}
	return res.sym, res.ok
}

// ResolveName resolves a single-segment (unqualified) reference from the given
// scope. The at node keys the memo table.
func (r *Resolver) ResolveName(scope *symbols.Scope, name string, at ast.Node) (*symbols.Symbol, bool) {
	if r.foreignScope(scope) {
		var sym *symbols.Symbol
		var ok bool
		r.aside(func() { sym, ok = r.ResolveName(scope, name, at) })
		return sym, ok
	}
	if at != nil {
		if res, done := r.memo[at]; done {
			return res.sym, res.ok
		}
	}
	mode, keyed := r.modeKey(at, nil)
	if keyed {
		if res, done := r.modeMemo[mode]; done {
			return res.sym, res.ok
		}
	}
	r.Enter()
	res := r.walkUnqualified(scope, name)
	settled := r.Leave()
	res.sym = r.AliasedElement(res.sym)
	if r.AliasNamesNothing(res.sym) {
		res = resolution{nil, false}
	}
	// A result found with the boundary lifted for an enclosing `import all` is
	// not what this reference resolves to in general, so it is not memoized.
	if keyed && (res.ok || r.quiet == 0) && settled {
		journalNew(r, r.modeMemo, mode, at)
		r.modeMemo[mode] = res
	}
	if (res.ok || r.quiet == 0) && r.allVisible == 0 && settled {
		r.memoize(at, res)
	}
	if !res.ok {
		r.report(Diagnostic{
			Span:    spanOf(at),
			Message: r.unresolvedMessage(scope, name, at),
			Fixes:   r.unresolvedFixes(scope, name, at),
		})
	}
	return res.sym, res.ok
}

// report records a diagnostic, unless the lookup that produced it was made for
// a semantic query rather than for a reference in this document (see aside).
func (r *Resolver) report(d Diagnostic) {
	if r.quiet > 0 {
		return
	}
	r.Diagnostics = append(r.Diagnostics, d)
}

// foreignScope reports whether scope belongs to a document other than the one
// being resolved, so a reference read there is that document's to report.
func (r *Resolver) foreignScope(scope *symbols.Scope) bool {
	if r.document == "" || r.quiet > 0 || scope == nil {
		return false
	}
	doc := r.documentOf(scope)
	return doc != "" && doc != r.document
}

// aside runs a lookup made for a semantic query, whose diagnostics belong to
// the document declaring what it reached, not to the one being resolved.
func (r *Resolver) aside(f func()) {
	r.quiet++
	defer func() { r.quiet-- }()
	f()
}

// inAllVisible runs f with every membership of a namespace reachable through
// it, as an `import all` and an expose resolve their target (KerML 8.2.3.5.2,
// SysML v2 8.3.26.2).
func (r *Resolver) inAllVisible(f func()) {
	r.allVisible++
	defer func() { r.allVisible-- }()
	f()
}

// outsideAllVisible runs f with the boundary reinstated, for a lookup made
// while an enclosing `import all` resolves its own target.
func (r *Resolver) outsideAllVisible(f func()) {
	saved := r.allVisible
	r.allVisible = 0
	defer func() { r.allVisible = saved }()
	f()
}

// probe runs a trial reading of qn, reported by f as resolved or not. Its
// diagnostics are suppressed, and the per-segment resolutions it records are
// kept only where it resolved: a reading not adopted leaves nothing behind for a
// caller to read back — unless qn is itself the name a ProbeReading reads.
func (r *Resolver) probe(qn *ast.QualifiedName, f func() bool) bool {
	saved := r.saveSegments(qn)
	resolved := false
	r.aside(func() { resolved = f() })
	if !resolved && qn != r.trial {
		r.restoreSegments(qn, saved)
	}
	return resolved
}

// segmentRecords are the per-segment records a walk of a qualified name leaves
// behind, copied so a later walk cannot alter them.
type segmentRecords struct {
	parts, aliases       []*symbols.Symbol
	hadParts, hadAliases bool
	ambiguity            int
	hadAmbiguity         bool
}

// saveSegments copies the records qn holds, for restoreSegments to put back.
func (r *Resolver) saveSegments(qn *ast.QualifiedName) segmentRecords {
	var saved segmentRecords
	if parts, had := r.parts[qn]; had {
		saved.parts, saved.hadParts = append([]*symbols.Symbol(nil), parts...), true
	}
	if aliases, had := r.aliasNames[qn]; had {
		saved.aliases, saved.hadAliases = append([]*symbols.Symbol(nil), aliases...), true
	}
	saved.ambiguity, saved.hadAmbiguity = r.ambiguities[qn]
	return saved
}

// restoreSegments puts back the records qn held when saved was taken.
func (r *Resolver) restoreSegments(qn *ast.QualifiedName, saved segmentRecords) {
	r.clearSegments(qn)
	if saved.hadParts {
		journalNew(r, r.parts, qn, qn)
		r.parts[qn] = saved.parts
	}
	if saved.hadAliases {
		journalNew(r, r.aliasNames, qn, qn)
		r.aliasNames[qn] = saved.aliases
	}
	if saved.hadAmbiguity {
		journalNew(r, r.ambiguities, qn, qn)
		r.ambiguities[qn] = saved.ambiguity
	}
}

// clearSegments drops the records qn holds, so a walk records it afresh.
func (r *Resolver) clearSegments(qn *ast.QualifiedName) {
	delete(r.parts, qn)
	delete(r.aliasNames, qn)
	delete(r.ambiguities, qn)
}

// modeKey keys a lookup made in a non-default mode, reporting false when the
// mode is the ordinary one, whose answers the main memo already keeps.
func (r *Resolver) modeKey(at ast.Node, hide *refFilter) (modeMemoKey, bool) {
	if at == nil || (r.inCondition == 0 && r.allVisible == 0) {
		return modeMemoKey{}, false
	}
	k := modeMemoKey{at: at, condition: r.inCondition > 0, allVisible: r.allVisible > 0}
	if hide != nil {
		k.hide, k.prefix, k.borrowedOut = hide.decl, hide.skipNamingTarget, hide.skipBorrowedName
	}
	return k, true
}

// memoize remembers a reference's resolution, except one reached while a filter
// condition's names were resolved: those lookups are unfiltered (see
// InCondition), and an ordinary reference must not inherit an unjudged answer.
func (r *Resolver) memoize(at ast.Node, res resolution) {
	if at == nil || r.inCondition > 0 {
		return
	}
	journalNew(r, r.memo, at, at)
	r.memo[at] = res
}

// memoizeFeatureChain caches a settled chain result unless condition or quiet
// rules forbid it.
func (r *Resolver) memoizeFeatureChain(scope *symbols.Scope, fc *ast.FeatureChainExpr, res resolution, settled bool) {
	if fc == nil || !settled || r.inCondition > 0 || (!res.ok && r.quiet > 0) {
		return
	}
	key := featureChainKey{scope: scope, node: fc}
	journalNew(r, r.featureChains, key, fc)
	r.featureChains[key] = res
}

// InCondition runs the resolution of a filter condition's own names, which its
// namespace's filters do not restrict: a condition naming a metadata type the
// namespace imports would otherwise be filtered by itself (KerML 8.2.4).
// Nothing resolved meanwhile is memoized, so the bypass reaches no other lookup.
func (r *Resolver) InCondition(f func()) {
	r.inCondition++
	defer func() { r.inCondition-- }()
	f()
}

func spanOf(n ast.Node) source.Span {
	if n == nil {
		return source.Span{}
	}
	return n.Span()
}

// qnText renders a qualified name for diagnostics (segments joined by "::",
// "$::" prefix when global).
func qnText(qn *ast.QualifiedName) string {
	s := ""
	for i, part := range qn.Parts {
		if i > 0 {
			s += "::"
		}
		s += part.Text
	}
	if qn.Global {
		s = "$::" + s
	}
	return s
}
