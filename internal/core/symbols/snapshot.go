package symbols

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast/astcodec"
	"github.com/Open-MBEE/OpenSysML/internal/core/pack"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// ErrNotSnapshottable reports an index WriteSnapshot cannot serialise: one
// still writable, built over another, or with an expansion pending.
var ErrNotSnapshottable = errors.New("symbols: only a frozen index with no base can be written as a snapshot")

// WriteSnapshot writes the index to w so that ReadSnapshot rebuilds the same
// frozen index: nodes, scopes and symbols are numbered and referenced by number.
func (idx *Index) WriteSnapshot(w *pack.Writer) error {
	if !idx.frozen || idx.base != nil || len(idx.dirtyNS) != 0 {
		return ErrNotSnapshottable
	}
	e := &snapshotEncoder{
		w:      w,
		scopes: make(map[*Scope]uint64),
		syms:   make(map[*Symbol]uint64),
		facts:  make(map[*LibraryFacts]uint64),
	}
	docs := idx.docRoots.keys()
	sort.Strings(docs)
	roots := make([]ast.Node, 0, len(docs))
	for _, doc := range docs {
		root := idx.docRoots.at(doc)
		roots = append(roots, root.node)
		e.collectScope(root)
	}
	idx.collectFromTables(e)

	e.ast = astcodec.NewEncoder(w)
	if err := e.ast.Encode(roots); err != nil {
		return err
	}
	w.Len(len(e.scopeList))
	w.Len(len(e.factList))
	w.Len(len(e.symList))
	e.section(w, e.writeScopes)
	e.section(w, e.writeSymbols)
	e.section(w, func() { e.writeTables(idx, docs) })
	return e.err
}

// section writes what write produces as a section of w.
func (e *snapshotEncoder) section(w *pack.Writer, write func()) {
	w.Section(func(sub *pack.Writer) {
		e.w = sub
		write()
	})
	e.w = w
}

type snapshotEncoder struct {
	w   *pack.Writer
	ast *astcodec.Encoder
	err error

	scopes    map[*Scope]uint64
	scopeList []*Scope
	syms      map[*Symbol]uint64
	symList   []*Symbol
	facts     map[*LibraryFacts]uint64
	factList  []*LibraryFacts
}

// collectScope numbers a scope tree and the symbols it registers.
func (e *snapshotEncoder) collectScope(s *Scope) {
	if s == nil || e.scopes[s] != 0 {
		return
	}
	e.scopes[s] = uint64(len(e.scopeList)) + 1
	e.scopeList = append(e.scopeList, s)
	for _, sym := range s.members {
		e.collectSymbol(sym)
	}
	for _, c := range s.children {
		e.collectScope(c)
	}
}

func (e *snapshotEncoder) collectSymbol(sym *Symbol) {
	if sym == nil || e.syms[sym] != 0 {
		return
	}
	e.syms[sym] = uint64(len(e.symList)) + 1
	e.symList = append(e.symList, sym)
	if sym.Facts != nil && e.facts[sym.Facts] == 0 {
		e.facts[sym.Facts] = uint64(len(e.factList)) + 1
		e.factList = append(e.factList, sym.Facts)
	}
	e.collectScope(sym.Scope)
	e.collectScope(sym.OwnerScope)
}

func (e *snapshotEncoder) collectFilters(filters []ElementFilter) {
	for _, f := range filters {
		e.collectScope(f.Scope)
	}
}

// collectFromTables numbers whatever a table refers to that no scope tree
// reached, so that every reference the tables make can be written.
func (idx *Index) collectFromTables(e *snapshotEncoder) {
	for _, syms := range idx.fqn.own {
		for _, s := range syms {
			e.collectSymbol(s)
		}
	}
	for _, entries := range idx.contributions.own {
		for _, en := range entries {
			e.collectSymbol(en.sym)
		}
	}
	for _, byDoc := range idx.wildcardMeta.own {
		for _, imports := range byDoc {
			for _, imp := range imports {
				e.collectScope(imp.Filter.Scope)
			}
		}
	}
	for _, m := range idx.reexported.own {
		for _, s := range m {
			e.collectSymbol(s)
		}
	}
	for _, m := range idx.hidden.own {
		for _, s := range m {
			e.collectSymbol(s)
		}
	}
	for k, byDoc := range idx.reexportDocs.own {
		e.collectSymbol(k.sym)
		for _, claim := range byDoc {
			for _, route := range claim.routes {
				e.collectFilters(route.filters)
			}
		}
	}
	for _, m := range idx.docReexports.own {
		for k := range m {
			e.collectSymbol(k.sym)
		}
	}
	for s := range idx.declaredAt.own {
		e.collectSymbol(s)
	}
	for s := range idx.librarySyms.own {
		e.collectSymbol(s)
	}
	for _, byDoc := range idx.nsFilters.own {
		for _, filters := range byDoc {
			e.collectFilters(filters)
		}
	}
	for _, syms := range idx.aboutUsages {
		for _, s := range syms {
			e.collectSymbol(s)
		}
	}
	for s := range idx.docOfRoot.own {
		e.collectScope(s)
	}
}

func (e *snapshotEncoder) scope(s *Scope) {
	if s == nil {
		e.w.Uint(0)
		return
	}
	id, ok := e.scopes[s]
	if !ok {
		e.fail("scope not collected")
	}
	e.w.Uint(id)
}

func (e *snapshotEncoder) sym(s *Symbol) {
	if s == nil {
		e.w.Uint(0)
		return
	}
	id, ok := e.syms[s]
	if !ok {
		e.fail("symbol not collected")
	}
	e.w.Uint(id)
}

// node writes a reference to an encoded syntax node; nil is 0.
func (e *snapshotEncoder) node(n ast.Node) {
	id, ok := e.ast.ID(n)
	if !ok {
		e.fail(fmt.Sprintf("%T not reached from a document root", n))
	}
	e.w.Uint(id)
}

func (e *snapshotEncoder) fail(what string) {
	if e.err == nil {
		e.err = fmt.Errorf("symbols: snapshot: %s", what)
	}
}

func (e *snapshotEncoder) span(s source.Span) {
	e.w.Int(int64(s.Offset))
	e.w.Int(int64(s.Len))
}

func (e *snapshotEncoder) symbols(syms []*Symbol) {
	e.w.Len(len(syms))
	for _, s := range syms {
		e.sym(s)
	}
}

func (e *snapshotEncoder) strings(ss []string) {
	e.w.Len(len(ss))
	for _, s := range ss {
		e.w.String(s)
	}
}

func (e *snapshotEncoder) writeScopes() {
	for _, s := range e.scopeList {
		e.scope(s.parent)
		e.sym(s.owner)
		e.node(s.node)
		e.w.Bool(s.bodyLocal)
		e.w.String(s.docName)
		e.strings(s.names)
		e.symbols(s.syms)
		e.symbols(s.members)
		e.symbols(s.anonymousMembers)
		e.w.Len(len(s.children))
		for _, c := range s.children {
			e.scope(c)
		}
	}
}

func (e *snapshotEncoder) writeSymbols() {
	for _, f := range e.factList {
		e.writeFacts(f)
	}
	for _, s := range e.symList {
		e.w.String(s.Name)
		e.w.Int(int64(s.Kind))
		e.node(s.Decl)
		e.w.Int(int64(s.Visibility))
		e.span(s.DeclSpan)
		e.span(s.NameSpan)
		e.scope(s.Scope)
		e.scope(s.OwnerScope)
		e.w.Len(len(s.LeadingTrivia))
		for _, t := range s.LeadingTrivia {
			e.w.Int(int64(t.Kind))
			e.span(t.Span)
		}
		e.w.String(s.DocName)
		if s.Facts == nil {
			e.w.Uint(0)
		} else {
			e.w.Uint(e.facts[s.Facts])
		}
		e.w.String(s.ShortName)
		e.w.Uint(uint64(s.Naming))
		e.node(s.NamingTarget)
	}
}

// writeFacts writes one LibraryFacts. A nil Supers differs from an empty one
// (see LibraryFacts), so the two are told apart.
func (e *snapshotEncoder) writeFacts(f *LibraryFacts) {
	e.w.Bool(f.Supers != nil)
	e.strings(f.Supers)
	e.w.Bool(f.Unit != nil)
	if f.Unit != nil {
		e.w.Float(f.Unit.ScaleNum)
		e.w.Float(f.Unit.ScaleDen)
		e.w.Len(len(f.Unit.Factors))
		for _, fac := range f.Unit.Factors {
			e.w.String(fac.FQN)
			e.w.Float(fac.Exponent)
		}
		e.w.Bool(f.Unit.Irreducible)
	}
	e.w.Bool(f.Dimension != nil)
	if f.Dimension != nil {
		e.w.Len(len(f.Dimension.Factors))
		for _, fac := range f.Dimension.Factors {
			e.w.String(fac.FQN)
			e.w.Float(fac.Exponent)
		}
	}
	e.w.Bool(f.Abstract)
}

func (e *snapshotEncoder) filter(f ElementFilter) {
	e.node(f.Expr)
	e.scope(f.Scope)
	e.span(f.Span)
}

// filters writes a filter list, keeping an empty one apart from none.
func (e *snapshotEncoder) filters(fs []ElementFilter) {
	e.w.Bool(fs != nil)
	e.w.Len(len(fs))
	for _, f := range fs {
		e.filter(f)
	}
}

// sortedSymbols returns the keys of a map in e's numbering order.
func sortedSymbols[V any](e *snapshotEncoder, m map[*Symbol]V) []*Symbol {
	syms := make([]*Symbol, 0, len(m))
	for s := range m {
		syms = append(syms, s)
	}
	sort.Slice(syms, func(i, j int) bool { return e.syms[syms[i]] < e.syms[syms[j]] })
	return syms
}

func (e *snapshotEncoder) sortedReexportKeys(m map[reexportKey]bool) []reexportKey {
	keys := make([]reexportKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	e.sortReexportKeys(keys)
	return keys
}

func (e *snapshotEncoder) sortReexportKeys(keys []reexportKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].fqn != keys[j].fqn {
			return keys[i].fqn < keys[j].fqn
		}
		return e.syms[keys[i].sym] < e.syms[keys[j].sym]
	})
}

func (e *snapshotEncoder) writeTables(idx *Index, docs []string) {
	// docRoots, docOfRoot
	e.w.Len(len(docs))
	for _, doc := range docs {
		e.w.String(doc)
		e.scope(idx.docRoots.own[doc])
	}
	roots := make([]*Scope, 0, len(idx.docOfRoot.own))
	for s := range idx.docOfRoot.own {
		roots = append(roots, s)
	}
	sort.Slice(roots, func(i, j int) bool { return e.scopes[roots[i]] < e.scopes[roots[j]] })
	e.w.Len(len(roots))
	for _, s := range roots {
		e.scope(s)
		e.w.String(idx.docOfRoot.own[s])
	}

	// docKinds
	keys := sortedKeys(idx.docKinds.own)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		e.w.Int(int64(idx.docKinds.own[k]))
	}

	// fqn
	keys = sortedKeys(idx.fqn.own)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		e.symbols(idx.fqn.own[k])
	}

	// contributions
	keys = sortedKeys(idx.contributions.own)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		entries := idx.contributions.own[k]
		e.w.Len(len(entries))
		for _, en := range entries {
			e.w.String(en.fqn)
			e.sym(en.sym)
		}
	}

	// wildcardMeta
	keys = sortedKeys(idx.wildcardMeta.own)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		byDoc := idx.wildcardMeta.own[k]
		docs := sortedKeys(byDoc)
		e.w.Len(len(docs))
		for _, doc := range docs {
			e.w.String(doc)
			imports := byDoc[doc]
			e.w.Len(len(imports))
			for _, imp := range imports {
				e.w.String(imp.Target)
				e.w.Bool(imp.Private)
				e.filter(imp.Filter)
			}
		}
	}

	// reexported, hidden
	for _, table := range []*layer[string, symbolSet]{idx.reexported, idx.hidden} {
		keys = sortedKeys(table.own)
		e.w.Len(len(keys))
		for _, k := range keys {
			e.w.String(k)
			e.symbols(table.own[k])
		}
	}

	// reexportDocs
	rkeys := make([]reexportKey, 0, len(idx.reexportDocs.own))
	for k := range idx.reexportDocs.own {
		rkeys = append(rkeys, k)
	}
	e.sortReexportKeys(rkeys)
	e.w.Len(len(rkeys))
	for _, k := range rkeys {
		e.w.String(k.fqn)
		e.sym(k.sym)
		byDoc := idx.reexportDocs.own[k]
		docs := sortedKeys(byDoc)
		e.w.Len(len(docs))
		for _, doc := range docs {
			e.w.String(doc)
			claim := byDoc[doc]
			e.w.Bool(claim.public)
			e.w.Len(len(claim.routes))
			for _, route := range claim.routes {
				e.w.Bool(route.private)
				e.filters(route.filters)
			}
		}
	}

	// docReexports
	keys = sortedKeys(idx.docReexports.own)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		rkeys := e.sortedReexportKeys(idx.docReexports.own[k])
		e.w.Len(len(rkeys))
		for _, rk := range rkeys {
			e.w.String(rk.fqn)
			e.sym(rk.sym)
		}
	}

	// declaredAt, librarySyms
	syms := make([]*Symbol, 0, len(idx.declaredAt.own))
	for s := range idx.declaredAt.own {
		syms = append(syms, s)
	}
	sort.Slice(syms, func(i, j int) bool { return e.syms[syms[i]] < e.syms[syms[j]] })
	e.w.Len(len(syms))
	for _, s := range syms {
		e.sym(s)
		e.w.String(idx.declaredAt.own[s])
	}
	syms = sortedSymbols(e, idx.librarySyms.own)
	e.w.Len(len(syms))
	for _, s := range syms {
		e.sym(s)
		e.w.Uint(uint64(idx.librarySyms.own[s]))
	}

	// children
	keys = sortedKeys(idx.children.own)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		e.strings(idx.children.own[k])
	}

	// bySegment
	keys = sortedKeys(idx.bySegment.own)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		e.strings(idx.bySegment.own[k])
	}

	// lastTargets
	keys = sortedKeys(idx.lastTargets.own)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		targets := idx.lastTargets.own[k]
		e.w.Len(len(targets))
		for _, t := range targets {
			e.w.String(t.doc)
			e.w.String(t.fqn)
			e.w.Bool(t.private)
		}
	}

	// libraryDocs
	keys = sortedKeys(idx.libraryDocs.own)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		e.w.Uint(uint64(idx.libraryDocs.own[k].Tier))
		e.w.String(idx.libraryDocs.own[k].Digest)
	}

	// nsFilters
	keys = sortedKeys(idx.nsFilters.own)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		byDoc := idx.nsFilters.own[k]
		docs := sortedKeys(byDoc)
		e.w.Len(len(docs))
		for _, doc := range docs {
			e.w.String(doc)
			e.filters(byDoc[doc])
		}
	}

	// aboutUsages
	keys = sortedKeys(idx.aboutUsages)
	e.w.Len(len(keys))
	for _, k := range keys {
		e.w.String(k)
		e.symbols(idx.aboutUsages[k])
	}
}

// ReadSnapshot rebuilds the frozen index WriteSnapshot wrote to r. Nodes, scopes
// and symbols are allocated up front, then the sections fill them concurrently.
func ReadSnapshot(r *pack.Reader) (*Index, error) {
	d := &snapshotDecoder{ast: astcodec.NewDecoder(r)}
	d.ast.Allocate()
	d.scopes = make([]Scope, r.Len())
	d.facts = make([]LibraryFacts, r.Len())
	d.syms = make([]Symbol, r.Len())
	scopes, syms, tables := r.Section(), r.Section(), r.Section()
	if err := r.Err(); err != nil {
		return nil, err
	}
	if !r.Done() {
		return nil, fmt.Errorf("%w: bytes after the index", pack.ErrCorrupt)
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); d.ast.Fill() }()
	go func() { defer wg.Done(); d.section(scopes).readScopes() }()
	go func() { defer wg.Done(); d.section(syms).readSymbols() }()
	idx := d.section(tables).readTables()
	wg.Wait()

	if err := d.ast.Err(); err != nil {
		return nil, err
	}
	for _, sec := range []*pack.Reader{scopes, syms, tables} {
		if err := sec.Err(); err != nil {
			return nil, err
		}
		if !sec.Done() {
			return nil, fmt.Errorf("%w: bytes after a section", pack.ErrCorrupt)
		}
	}
	return idx, nil
}

// snapshotDecoder holds the blocks every section refers into.
type snapshotDecoder struct {
	ast    *astcodec.Decoder
	scopes []Scope
	syms   []Symbol
	facts  []LibraryFacts
}

// sectionReader decodes one section; its arenas are its own, so sections can
// be read concurrently.
type sectionReader struct {
	*snapshotDecoder
	r *pack.Reader

	symSlices    pack.Arena[*Symbol]
	scopeSlices  pack.Arena[*Scope]
	stringSlices pack.Arena[string]
	trivia       pack.Arena[ast.Trivia]
	entries      pack.Arena[fqnEntry]
	filterSlices pack.Arena[ElementFilter]
}

func (d *snapshotDecoder) section(r *pack.Reader) *sectionReader {
	return &sectionReader{snapshotDecoder: d, r: r}
}

// node reads a reference to a syntax node; 0 is nil.
func (d *sectionReader) node() ast.Node {
	n, ok := d.ast.Node(d.r.Uint())
	if !ok {
		d.r.Fail("node index")
	}
	return n
}

func (d *sectionReader) scope() *Scope {
	id := d.r.Uint()
	if id == 0 {
		return nil
	}
	if id > uint64(len(d.scopes)) {
		d.r.Fail("scope index")
		return nil
	}
	return &d.scopes[id-1]
}

func (d *sectionReader) sym() *Symbol {
	id := d.r.Uint()
	if id == 0 {
		return nil
	}
	if id > uint64(len(d.syms)) {
		d.r.Fail("symbol index")
		return nil
	}
	return &d.syms[id-1]
}

func (d *sectionReader) span() source.Span {
	return source.Span{Offset: int(d.r.Int()), Len: int(d.r.Int())}
}

func (d *sectionReader) symbols() []*Symbol {
	out := d.symSlices.Take(d.r.Len())
	for i := range out {
		out[i] = d.sym()
	}
	return out
}

func (d *sectionReader) strings() []string {
	out := d.stringSlices.Take(d.r.Len())
	for i := range out {
		out[i] = d.r.String()
	}
	return out
}

func (d *sectionReader) filter() ElementFilter {
	var f ElementFilter
	f.Expr = d.node()
	f.Scope = d.scope()
	f.Span = d.span()
	return f
}

func (d *sectionReader) filters() []ElementFilter {
	present := d.r.Bool()
	out := d.filterSlices.Take(d.r.Len())
	if present && out == nil {
		out = []ElementFilter{}
	}
	for i := range out {
		out[i] = d.filter()
	}
	return out
}

func (d *sectionReader) readScopes() {
	for i := range d.scopes {
		s := &d.scopes[i]
		s.parent = d.scope()
		s.owner = d.sym()
		s.node = d.node()
		s.bodyLocal = d.r.Bool()
		s.docName = d.r.String()
		s.names = d.strings()
		s.syms = d.symbols()
		s.members = d.symbols()
		s.anonymousMembers = d.symbols()
		s.children = d.scopeSlices.Take(d.r.Len())
		for j := range s.children {
			s.children[j] = d.scope()
		}
		if d.r.Err() != nil {
			return
		}
	}
}

func (d *sectionReader) readSymbols() {
	for i := range d.facts {
		d.readFacts(&d.facts[i])
	}
	for i := range d.syms {
		s := &d.syms[i]
		s.Name = d.r.String()
		s.Kind = SymbolKind(d.r.Int())
		s.Decl = d.node()
		s.Visibility = ast.Visibility(d.r.Int())
		s.DeclSpan = d.span()
		s.NameSpan = d.span()
		s.Scope = d.scope()
		s.OwnerScope = d.scope()
		s.LeadingTrivia = d.trivia.Take(d.r.Len())
		for j := range s.LeadingTrivia {
			s.LeadingTrivia[j].Kind = ast.TriviaKind(d.r.Int())
			s.LeadingTrivia[j].Span = d.span()
		}
		s.DocName = d.r.String()
		if id := d.r.Uint(); id != 0 {
			if id > uint64(len(d.facts)) {
				d.r.Fail("facts index")
				return
			}
			s.Facts = &d.facts[id-1]
		}
		s.ShortName = d.r.String()
		naming := d.r.Uint()
		if naming > uint64(NamedByRedefinition) {
			d.r.Fail("naming kind")
			return
		}
		s.Naming = Naming(naming)
		s.NamingTarget = d.node()
		if d.r.Err() != nil {
			return
		}
	}
}

func (d *sectionReader) readFacts(f *LibraryFacts) {
	hasSupers := d.r.Bool()
	f.Supers = d.strings()
	if hasSupers && f.Supers == nil {
		f.Supers = []string{}
	}
	if d.r.Bool() {
		u := &UnitFacts{ScaleNum: d.r.Float(), ScaleDen: d.r.Float()}
		if n := d.r.Len(); n > 0 {
			u.Factors = make([]UnitFactorFacts, n)
		}
		for i := range u.Factors {
			u.Factors[i].FQN = d.r.String()
			u.Factors[i].Exponent = d.r.Float()
		}
		u.Irreducible = d.r.Bool()
		f.Unit = u
	}
	if d.r.Bool() {
		dim := &DimensionFacts{}
		if n := d.r.Len(); n > 0 {
			dim.Factors = make([]DimensionFactorFacts, n)
		}
		for i := range dim.Factors {
			dim.Factors[i].FQN = d.r.String()
			dim.Factors[i].Exponent = d.r.Float()
		}
		f.Dimension = dim
	}
	f.Abstract = d.r.Bool()
}

// stringTable reads a string-keyed table of n entries, each value read by
// value, into a layer with nothing below it.
func stringTable[V any](d *sectionReader, gen *indexGeneration, value func() V) *layer[string, V] {
	n := d.r.Len()
	m := make(map[string]V, n)
	for i := 0; i < n; i++ {
		k := d.r.String()
		m[k] = value()
	}
	return &layer[string, V]{own: m, gen: gen}
}

func (d *sectionReader) symbolSet() symbolSet {
	set := make(symbolSet, d.r.Len())
	for i := range set {
		set[i] = d.sym()
	}
	return set
}

// libraryTier reads a tier, refusing one this build does not know.
func (d *sectionReader) libraryTier() LibraryTier {
	v := d.r.Uint()
	if v >= uint64(numLibraryTiers) {
		d.r.Fail("library tier")
		return TierNone
	}
	return LibraryTier(v)
}

func (d *sectionReader) reexportKey() reexportKey {
	return reexportKey{fqn: d.r.String(), sym: d.sym()}
}

func (d *sectionReader) readTables() *Index {
	idx := NewIndex()
	gen := idx.generation

	n := d.r.Len()
	docRoots := make(map[string]*Scope, n)
	for i := 0; i < n; i++ {
		doc := d.r.String()
		docRoots[doc] = d.scope()
	}
	idx.docRoots = &layer[string, *Scope]{own: docRoots, gen: gen}
	n = d.r.Len()
	docOfRoot := make(map[*Scope]string, n)
	for i := 0; i < n; i++ {
		s := d.scope()
		docOfRoot[s] = d.r.String()
	}
	idx.docOfRoot = &layer[*Scope, string]{own: docOfRoot, gen: gen}

	idx.docKinds = stringTable(d, gen, func() source.Kind { return source.Kind(d.r.Int()) })
	idx.fqn = stringTable(d, gen, d.symbols)
	idx.contributions = stringTable(d, gen, func() []fqnEntry {
		out := d.entries.Take(d.r.Len())
		for i := range out {
			out[i].fqn = d.r.String()
			out[i].sym = d.sym()
		}
		return out
	})
	idx.wildcardMeta = stringTable(d, gen, func() map[string][]WildcardImport {
		n := d.r.Len()
		byDoc := make(map[string][]WildcardImport, n)
		for i := 0; i < n; i++ {
			doc := d.r.String()
			imports := make([]WildcardImport, d.r.Len())
			for j := range imports {
				imports[j].Target = d.r.String()
				imports[j].Private = d.r.Bool()
				imports[j].Filter = d.filter()
			}
			byDoc[doc] = imports
		}
		return byDoc
	})
	idx.reexported = stringTable(d, gen, d.symbolSet)
	idx.hidden = stringTable(d, gen, d.symbolSet)

	n = d.r.Len()
	reexportDocs := make(map[reexportKey]map[string]*reexportClaim, n)
	claims := make([]reexportClaim, 0, n)
	for i := 0; i < n; i++ {
		k := d.reexportKey()
		m := d.r.Len()
		byDoc := make(map[string]*reexportClaim, m)
		for j := 0; j < m; j++ {
			doc := d.r.String()
			claim := reexportClaim{public: d.r.Bool()}
			claim.routes = make([]gateRoute, d.r.Len())
			for r := range claim.routes {
				claim.routes[r].private = d.r.Bool()
				claim.routes[r].filters = d.filters()
			}
			if len(claims) == cap(claims) {
				claims = make([]reexportClaim, 0, n)
			}
			claims = append(claims, claim)
			byDoc[doc] = &claims[len(claims)-1]
		}
		reexportDocs[k] = byDoc
		if d.r.Err() != nil {
			return idx
		}
	}
	idx.reexportDocs = &layer[reexportKey, map[string]*reexportClaim]{own: reexportDocs, gen: gen}

	idx.docReexports = stringTable(d, gen, func() map[reexportKey]bool {
		n := d.r.Len()
		m := make(map[reexportKey]bool, n)
		for i := 0; i < n; i++ {
			m[d.reexportKey()] = true
		}
		return m
	})

	n = d.r.Len()
	declaredAt := make(map[*Symbol]string, n)
	for i := 0; i < n; i++ {
		s := d.sym()
		declaredAt[s] = d.r.String()
	}
	idx.declaredAt = &layer[*Symbol, string]{own: declaredAt, gen: gen}
	n = d.r.Len()
	librarySyms := make(map[*Symbol]LibraryTier, n)
	for i := 0; i < n; i++ {
		s := d.sym()
		librarySyms[s] = d.libraryTier()
	}
	idx.librarySyms = &layer[*Symbol, LibraryTier]{own: librarySyms, gen: gen}

	idx.children = stringTable(d, gen, d.strings)
	idx.bySegment = stringTable(d, gen, d.strings)
	idx.lastTargets = stringTable(d, gen, func() []resolvedImport {
		out := make([]resolvedImport, d.r.Len())
		for i := range out {
			out[i].doc = d.r.String()
			out[i].fqn = d.r.String()
			out[i].private = d.r.Bool()
		}
		return out
	})
	idx.libraryDocs = stringTable(d, gen, func() LibraryDocument {
		return LibraryDocument{Tier: d.libraryTier(), Digest: d.r.String()}
	})
	idx.nsFilters = stringTable(d, gen, func() map[string][]ElementFilter {
		n := d.r.Len()
		byDoc := make(map[string][]ElementFilter, n)
		for i := 0; i < n; i++ {
			doc := d.r.String()
			byDoc[doc] = d.filters()
		}
		return byDoc
	})

	n = d.r.Len()
	idx.aboutUsages = make(map[string][]*Symbol, n)
	for i := 0; i < n; i++ {
		doc := d.r.String()
		idx.aboutUsages[doc] = d.symbols()
	}
	idx.takeLibraryIdentity()
	idx.frozen = true
	return idx
}
