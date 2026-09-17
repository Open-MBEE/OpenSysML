package resolve

import (
	"reflect"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// frame owns what the resolver and the side tables joining its lifecycle
// memoize while it is the innermost frame: a document's, or a Scratch call's.
type frame struct {
	// doc names the document whose nodes, symbols and scopes the entries are
	// keyed by; "" is the frame for state no document owns.
	doc string
	// transient is set for a Scratch frame: the nodes it disowns when it ends.
	transient map[ast.Node]bool
	// barrier is set for the frame Untracked pushes: nothing below it is current.
	barrier bool
	// journal is what the frame drops when it ends; ledgers finds the ledger
	// among it for a memo table, by the table's identity.
	journal []dropper
	ledgers map[uintptr]dropper
	// names, namespaces and docs are what the index answered about while the
	// frame was innermost; all is set once it enumerated the whole name table,
	// after which only names outside it (a judgment's) are worth recording.
	names      map[string]bool
	namespaces map[string]bool
	docs       map[string]bool
	all        bool
	// deps are the documents whose frames were entered from this one.
	deps map[string]bool
	// recent are the frames last entered from this one, consulted before the
	// maps: an analysis enters a library's frame once per symbol it reads there.
	recent [4]*frame
	next   uint8
}

func (f *frame) scratch() bool { return f.transient != nil }

func (f *frame) drop() {
	for _, d := range f.journal {
		d.drop()
	}
}

// entries counts what the frame will drop.
func (f *frame) entries() int {
	n := 0
	for _, d := range f.journal {
		n += d.size()
	}
	return n
}

// A dropper is an entry of a frame's journal.
type dropper interface {
	drop()
	size() int
}

// dropFunc is a journaled closure.
type dropFunc func()

func (d dropFunc) drop()     { d() }
func (d dropFunc) size() int { return 1 }

// ledger is a frame's share of one memo table: the keys it wrote there.
type ledger[K comparable, V any] struct {
	table map[K]V
	keys  []K
}

func (l *ledger[K, V]) drop() {
	for _, k := range l.keys {
		delete(l.table, k)
	}
}

func (l *ledger[K, V]) size() int { return len(l.keys) }

// entered is the frame of doc among those recently entered from f, or nil.
func (f *frame) entered(doc string) *frame {
	for _, g := range f.recent {
		if g != nil && g.doc == doc {
			return g
		}
	}
	return nil
}

func (f *frame) remember(g *frame) {
	f.recent[f.next%uint8(len(f.recent))] = g
	f.next++
}

// stale reports whether ch moved anything the frame's entries were read from.
func (f *frame) stale(ch symbols.Changes) bool {
	if ch.Docs[f.doc] || (f.all && ch.Registered()) {
		return true
	}
	if len(f.names) < len(ch.Names) {
		for n := range f.names {
			if ch.Names[n] {
				return true
			}
		}
	} else {
		for n := range ch.Names {
			if f.names[n] {
				return true
			}
		}
	}
	for n := range ch.Namespaces {
		if f.namespaces[n] {
			return true
		}
	}
	for n := range ch.Docs {
		if f.docs[n] {
			return true
		}
	}
	return false
}

// Track makes the resolver keep what it memoizes by owning document, so
// Invalidate can drop a document's entries and its dependents' when the index
// changes. The resolver records what it reads from the index from here on.
func (r *Resolver) Track() {
	if r.owners != nil {
		return
	}
	r.owners = map[string]*frame{}
	r.dependents = map[string]map[string]bool{}
	r.idx.TrackChanges()
	r.idx.SetReadRecorder(r)
}

// Tracking reports whether memo entries are journaled by owning document.
func (r *Resolver) Tracking() bool { return r != nil && r.owners != nil }

// Regatherer is a model keeping per-document gathers: Regather recomputes those
// of the documents that changed and names the shared state whose readers have
// to be dropped (see Invalidate).
type Regatherer interface {
	Regather(docs map[string]bool) (changed []string)
}

// InDocument runs f, the analysis of doc: what it memoizes about doc's own
// nodes is owned by doc, and the documents it reads are doc's dependencies.
// Failures resolved here are reported to f and memoized as reported, so the
// analysis must be the first to resolve doc's references (see Query).
func (r *Resolver) InDocument(doc string, f func()) {
	if !r.Tracking() {
		f()
		return
	}
	r.Diagnostics = nil
	r.EnterDoc(doc)
	defer r.LeaveDoc()
	f()
}

// Query runs f, a query made from doc — a hover, a reference, a completion —
// quietly: a failure it resolves is neither reported nor memoized, so doc's
// analysis still reports it. "" is the frame for a query made from no document.
func (r *Resolver) Query(doc string, f func()) {
	if !r.Tracking() {
		f()
		return
	}
	r.EnterDoc(doc)
	defer r.LeaveDoc()
	r.aside(f)
}

// gatherSuffix marks the frame a document's gather runs in (see Gather).
const gatherSuffix = "\x00gather"

// GatherFrame names the frame doc's gather runs in: apart from the frame of
// doc's analysis, so a judgment dropped for an answer it read does not take
// the gather it had no part in with it.
func GatherFrame(doc string) string { return doc + gatherSuffix }

// GatheredDoc is the document whose gather frame the name is, if it is one.
func GatheredDoc(frame string) (string, bool) {
	return strings.CutSuffix(frame, gatherSuffix)
}

// Gather runs f, the gathering of doc's facts for a workspace-wide judgment:
// in doc's gather frame, quiet like a Query, and no dependency of the
// enclosing document, whose judgment reads the union of gathers by name instead.
// Invalidate names the gather frames it drops, for the gathers to be redone.
func (r *Resolver) Gather(doc string, f func()) {
	if !r.Tracking() {
		f()
		return
	}
	r.Untracked(func() {
		r.EnterDoc(GatherFrame(doc))
		defer r.LeaveDoc()
		r.aside(f)
	})
}

// docFrame is the frame owning doc, made on first use.
func (r *Resolver) docFrame(doc string) *frame {
	f := r.owners[doc]
	if f == nil {
		f = &frame{doc: doc, all: doc == ""}
		r.owners[doc] = f
	}
	return f
}

// EnterDoc makes doc the owner of what is memoized until the matching
// LeaveDoc, and records that the enclosing document depends on doc. It is a
// no-op outside Track, so callers pair it with LeaveDoc unconditionally.
func (r *Resolver) EnterDoc(doc string) {
	if !r.Tracking() {
		return
	}
	cur := r.cur
	if doc == "" && cur != nil {
		// Unstamped state stays with whoever computed it.
		r.stack = append(r.stack, cur)
		return
	}
	if cur != nil && cur.doc == doc {
		r.stack = append(r.stack, cur)
		return
	}
	var f *frame
	if cur != nil {
		f = cur.entered(doc)
	}
	if f == nil {
		f = r.docFrame(doc)
		if cur != nil {
			r.depend(cur, doc)
			cur.remember(f)
		}
	}
	r.stack = append(r.stack, f)
	r.cur = f
}

// LeaveDoc ends the innermost EnterDoc.
func (r *Resolver) LeaveDoc() {
	if !r.Tracking() {
		return
	}
	n := len(r.stack) - 1
	r.stack[n] = nil
	r.stack = r.stack[:n]
	r.cur = r.topDoc()
}

// topDoc is the innermost document frame on the stack, nil when none or when
// a barrier is nearer.
func (r *Resolver) topDoc() *frame {
	for i := len(r.stack) - 1; i >= 0; i-- {
		switch f := r.stack[i]; {
		case f.barrier:
			return nil
		case !f.scratch():
			return f
		}
	}
	return nil
}

// Untracked runs f with no frame current: what it reads is recorded nowhere and
// what it memoizes is owned by no document. For reads whose answer is kept
// current by other means, such as the list of documents a gather covers.
func (r *Resolver) Untracked(f func()) {
	if !r.Tracking() {
		f()
		return
	}
	r.stack = append(r.stack, &frame{barrier: true})
	r.cur = nil
	defer r.LeaveDoc()
	f()
}

// Depend records that the enclosing document depends on doc: a symbol of doc
// was read, so doc's replacement invalidates what was computed from it.
func (r *Resolver) Depend(doc string) {
	if r == nil {
		return
	}
	cur := r.cur
	if cur == nil || doc == "" || doc == cur.doc || cur.entered(doc) != nil {
		return
	}
	r.depend(cur, doc)
	cur.remember(r.docFrame(doc))
}

// found returns a resolution, making the current document depend on the one
// the symbol it found was declared in.
func (r *Resolver) found(res resolution) (*symbols.Symbol, bool) {
	if res.sym != nil {
		r.Depend(res.sym.DocName)
	}
	return res.sym, res.ok
}

func (r *Resolver) depend(from *frame, doc string) {
	if from.doc == doc {
		return
	}
	if from.deps == nil {
		from.deps = map[string]bool{}
	}
	if from.deps[doc] {
		return
	}
	from.deps[doc] = true
	back := r.dependents[doc]
	if back == nil {
		back = map[string]bool{}
		r.dependents[doc] = back
	}
	back[from.doc] = true
}

// ReadName, ReadNamespace, ReadDocument and ReadAllNames implement
// symbols.ReadRecorder: what the index answered is what the frame depends on.
func (r *Resolver) ReadName(fqn string) {
	if r == nil {
		return
	}
	if f := r.cur; f != nil {
		if f.names == nil {
			f.names = map[string]bool{}
		}
		f.names[fqn] = true
	}
}

func (r *Resolver) ReadNamespace(fqn string) {
	if r == nil {
		return
	}
	if f := r.cur; f != nil && !f.all {
		if f.namespaces == nil {
			f.namespaces = map[string]bool{}
		}
		f.namespaces[fqn] = true
	}
}

func (r *Resolver) ReadDocument(name string) {
	if r == nil {
		return
	}
	if f := r.cur; f != nil && !f.all && name != f.doc {
		if f.docs == nil {
			f.docs = map[string]bool{}
		}
		f.docs[name] = true
	}
}

func (r *Resolver) ReadAllNames() {
	if r == nil {
		return
	}
	if f := r.cur; f != nil {
		f.all = true
	}
}

// Invalidate drops what the documents ch touched own, and what every document
// depending on them owns, transitively. It returns the documents dropped,
// sorted; each is analyzed afresh on its next request.
func (r *Resolver) Invalidate(ch symbols.Changes) []string {
	if !r.Tracking() || ch.Empty() {
		return nil
	}
	if g, ok := r.model.(Regatherer); ok && len(ch.Docs) > 0 {
		for _, name := range g.Regather(ch.Docs) {
			if ch.Names == nil {
				ch.Names = map[string]bool{}
			}
			ch.Names[name] = true
		}
	}
	if r.names != nil && ch.Registered() {
		r.names.Refresh(ch.Names)
	}
	var work []*frame
	for _, f := range r.owners {
		if f.stale(ch) {
			work = append(work, f)
		}
	}
	dropped := map[string]bool{}
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		if dropped[f.doc] {
			continue
		}
		dropped[f.doc] = true
		for dep := range r.dependents[f.doc] {
			if g := r.owners[dep]; g != nil && !dropped[dep] {
				work = append(work, g)
			}
		}
	}
	out := make([]string, 0, len(dropped))
	for doc := range dropped {
		r.dropFrame(doc)
		out = append(out, doc)
	}
	sort.Strings(out)
	return out
}

// InvalidateAll drops every document's entries.
func (r *Resolver) InvalidateAll() {
	if !r.Tracking() {
		return
	}
	for doc := range r.owners {
		r.dropFrame(doc)
	}
	r.idx.TakeChanges()
}

func (r *Resolver) dropFrame(doc string) {
	f := r.owners[doc]
	if f == nil {
		return
	}
	f.drop()
	for dep := range f.deps {
		delete(r.dependents[dep], doc)
	}
	delete(r.owners, doc)
}

// Dependents reports the documents whose state was computed from doc's,
// directly; for tests and diagnostics of the relation.
func (r *Resolver) Dependents(doc string) []string {
	out := make([]string, 0, len(r.dependents[doc]))
	for d := range r.dependents[doc] {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// Owned reports how many entries doc's frame will drop; for tests.
func (r *Resolver) Owned(doc string) int {
	if f := r.owners[doc]; f != nil {
		return f.entries()
	}
	return 0
}

// Scratch runs f, then forgets what f memoized about the transient nodes, so
// syntax the model does not own (a request expression) is not retained.
func (r *Resolver) Scratch(transient map[ast.Node]bool, f func()) {
	fr := &frame{transient: transient}
	r.stack = append(r.stack, fr)
	r.scratching++
	defer func() {
		r.scratching--
		n := len(r.stack) - 1
		r.stack[n] = nil
		r.stack = r.stack[:n]
		fr.drop()
	}()
	f()
}

// Journal registers drop to run when the frame owning node ends: the Scratch
// that disowns node, else the innermost document frame. It is how a side
// table keyed by node joins the resolver's lifecycle.
func (r *Resolver) Journal(node ast.Node, drop func()) {
	if f := r.owner(node); f != nil {
		f.journal = append(f.journal, dropFunc(drop))
	}
}

// owner is the frame whose journal an entry for node joins: the Scratch that
// disowns node, else the innermost document frame; nil when none does.
func (r *Resolver) owner(node ast.Node) *frame {
	if !r.Journaling() {
		return nil
	}
	if r.scratching > 0 {
		for i := len(r.stack) - 1; i >= 0; i-- {
			if f := r.stack[i]; f.scratch() && f.transient[node] {
				return f
			}
		}
	}
	switch {
	case r.cur != nil:
		return r.cur
	case len(r.stack) == 0 && r.Tracking():
		// Written outside any frame: owned by the frame every change drops.
		return r.docFrame("")
	}
	return nil
}

// Journaling reports whether a memo write now would be journaled, so a side
// table can skip building the drop closure when it would not.
func (r *Resolver) Journaling() bool {
	return r != nil && (len(r.stack) > 0 || r.Tracking())
}

// JournalNew journals the deletion of m[k], about to be written for node the
// first time, with the innermost frame: in that frame's ledger for m.
func JournalNew[K comparable, V any](r *Resolver, m map[K]V, k K, node ast.Node) {
	if !r.Journaling() {
		return
	}
	if _, had := m[k]; had {
		return
	}
	f := r.owner(node)
	if f == nil {
		return
	}
	id := reflect.ValueOf(m).Pointer()
	l, ok := f.ledgers[id].(*ledger[K, V])
	if !ok {
		l = &ledger[K, V]{table: m}
		if f.ledgers == nil {
			f.ledgers = map[uintptr]dropper{}
		}
		f.ledgers[id] = l
		f.journal = append(f.journal, l)
	}
	l.keys = append(l.keys, k)
}

// journalNew is JournalNew for the resolver's own tables.
func journalNew[K comparable, V any](r *Resolver, m map[K]V, k K, node ast.Node) {
	JournalNew(r, m, k, node)
}
