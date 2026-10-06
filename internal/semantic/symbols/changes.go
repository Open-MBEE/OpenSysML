package symbols

import (
	"fmt"
	"sort"
	"strings"
)

// Changes is what a run of writes to an index changed, at the granularity a ReadRecorder
// records reads at: names, namespaces and documents. Reads meeting none saw nothing move.
type Changes struct {
	Names      map[string]bool
	Namespaces map[string]bool
	Docs       map[string]bool
}

// Empty reports whether nothing changed.
func (c Changes) Empty() bool {
	return len(c.Names) == 0 && len(c.Namespaces) == 0 && len(c.Docs) == 0
}

// Registered reports whether the index's own tables changed: every write to
// them records a namespace or a document, so names alone are a caller's.
func (c Changes) Registered() bool {
	return len(c.Namespaces) > 0 || len(c.Docs) > 0
}

func newChanges() *Changes {
	return &Changes{Names: map[string]bool{}, Namespaces: map[string]bool{}, Docs: map[string]bool{}}
}

// TrackChanges starts recording what writes to the index change, for
// TakeChanges to hand out. An index that is never asked records nothing.
func (idx *Index) TrackChanges() {
	if idx.changes == nil {
		idx.changes = newChanges()
	}
}

// TakeChanges returns what changed since the last call and starts over. A
// name registered again exactly as it was (the same symbols, re-exported and
// hidden alike, claimed by the same documents on the same routes) is left
// out: a document replaced by one declaring the same names re-registers every
// re-export its wildcard imports surface, and none of them reads differently.
func (idx *Index) TakeChanges() Changes {
	if idx.changes == nil {
		return Changes{}
	}
	out := *idx.changes
	for fqn, before := range idx.changesBefore {
		if out.Names[fqn] && registrationOf(idx, fqn) == before {
			delete(out.Names, fqn)
		}
	}
	idx.changes = newChanges()
	idx.changesBefore = nil
	return out
}

// noteBefore keeps how fqn was registered before the first write to it since
// the last TakeChanges, for TakeChanges to tell a name that changed from one
// registered again as it was. Callers note before they write.
func (idx *Index) noteBefore(fqn string) {
	if idx.changes == nil {
		return
	}
	if _, noted := idx.changesBefore[fqn]; noted {
		return
	}
	if idx.changesBefore == nil {
		idx.changesBefore = map[string]string{}
	}
	idx.changesBefore[fqn] = registrationOf(idx, fqn)
}

// registrationOf spells everything a lookup of fqn reads from the index's
// tables: the symbols registered under it, in order, each with its re-export
// and hidden marks and the claims and routes that surfaced it. Symbols compare
// by identity, so a declaration parsed again is a change.
func registrationOf(idx *Index, fqn string) string {
	var b strings.Builder
	for _, sym := range idx.fqn.at(fqn) {
		reexported, _ := idx.reexported.get(fqn)
		hidden, _ := idx.hidden.get(fqn)
		fmt.Fprintf(&b, "%p %t %t;", sym, reexported.has(sym), hidden.has(sym))
		claims := idx.reexportDocs.at(reexportKey{fqn: fqn, sym: sym})
		docs := make([]string, 0, len(claims))
		for doc := range claims {
			docs = append(docs, doc)
		}
		sort.Strings(docs)
		for _, doc := range docs {
			claim := claims[doc]
			fmt.Fprintf(&b, "%q %t", doc, claim.public)
			for _, route := range claim.routes {
				fmt.Fprintf(&b, " %t", route.private)
				for _, filter := range route.filters {
					fmt.Fprintf(&b, " %p %p", filter.Expr, filter.Scope)
				}
			}
			b.WriteByte(';')
		}
		b.WriteByte('|')
	}
	return b.String()
}

func (idx *Index) changedName(fqn string) {
	if idx.changes == nil {
		return
	}
	idx.changes.Names[fqn] = true
	parent, _ := splitFQN(fqn)
	idx.changes.Namespaces[parent] = true
}

func (idx *Index) changedNamespace(fqn string) {
	if idx.changes == nil {
		return
	}
	idx.changes.Namespaces[fqn] = true
}

func (idx *Index) changedDoc(name string) {
	if idx.changes == nil {
		return
	}
	idx.changes.Docs[name] = true
}

// A ReadRecorder is told what an index read is about — a name looked up, a namespace
// enumerated or its children read, a document's root or kind, the whole table.
type ReadRecorder interface {
	ReadName(fqn string)
	ReadNamespace(fqn string)
	ReadDocument(name string)
	ReadAllNames()
	ReadSegment(name string)
}

// SetReadRecorder installs the recorder the index's reads report to, or none.
func (idx *Index) SetReadRecorder(r ReadRecorder) {
	idx.reads = r
}

// Recording returns a view of the index whose reads report to r: the same
// tables, caches and change log as idx, so a batch worker reading the index
// beside others records what its analysis read without one recorder for all.
// The view is for reading only; a write to it is a write to idx.
func (idx *Index) Recording(r ReadRecorder) *Index {
	view := *idx
	view.reads = r
	return &view
}

func (idx *Index) readName(fqn string) {
	if idx.reads != nil {
		idx.reads.ReadName(fqn)
	}
}

func (idx *Index) readNamespace(fqn string) {
	if idx.reads != nil {
		idx.reads.ReadNamespace(fqn)
	}
}

func (idx *Index) readDocument(name string) {
	if idx.reads != nil {
		idx.reads.ReadDocument(name)
	}
}

func (idx *Index) readAllNames() {
	if idx.reads != nil {
		idx.reads.ReadAllNames()
	}
}

func (idx *Index) readSegment(name string) {
	if idx.reads != nil {
		idx.reads.ReadSegment(name)
	}
}
