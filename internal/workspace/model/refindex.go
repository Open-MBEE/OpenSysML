package model

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ReferenceLocation is one written name segment in a workspace document, with
// the document text its span addresses (the bytes the index was built from).
type ReferenceLocation struct {
	Doc     string
	Content []byte
	Span    source.Span
}

// refEntry is a segment listed under an element: one it reaches, one whose name it
// writes (an alias's own name), or both when the name is the element's own.
// text is the name as written, telling a long-name spelling from a short one;
// ref and part locate the segment in its reference, for a rename's capture check.
type refEntry struct {
	ReferenceLocation
	text    string
	reached bool
	named   bool
	ref     resolve.Reference
	part    int
}

// refIndex maps every element (by symbols.KeyOf) to the segments in the
// workspace's documents — never the library's — that reach it or write its name,
// one table per document so a change drops only the documents it moved.
type refIndex struct {
	docs map[string]map[symbols.ElementKey][]refEntry
}

func newRefIndex() *refIndex {
	return &refIndex{docs: map[string]map[symbols.ElementKey][]refEntry{}}
}

// drop forgets doc's table; the next query rebuilds it. A nil index holds none.
func (x *refIndex) drop(doc string) {
	if x != nil {
		delete(x.docs, doc)
	}
}

// add records segment part of ref, which reaches element and writes name (either
// may be nil), in doc's table.
func (x *refIndex) add(doc *Document, ref resolve.Reference, part int, element, name *symbols.Symbol) {
	seg := ref.QN.Parts[part]
	loc := ReferenceLocation{Doc: doc.Name, Content: doc.Content, Span: seg.Span}
	entries := x.docs[doc.Name]
	put := func(sym *symbols.Symbol, reached, named bool) {
		key := symbols.KeyOf(sym)
		entries[key] = append(entries[key], refEntry{ReferenceLocation: loc, text: seg.Text,
			reached: reached, named: named, ref: ref, part: part})
	}
	switch {
	case element == nil && name == nil:
		return
	case symbols.SameElement(element, name):
		put(element, true, true)
		return
	}
	if element != nil {
		put(element, true, false)
	}
	if name != nil {
		put(name, false, true)
	}
}

// referencesLocked returns the reverse-index entries for key across every
// document, in document then position order, building the table of each
// document a change has dropped. Caller holds the write lock.
func (w *Workspace) referencesLocked(key symbols.ElementKey) []refEntry {
	if w.refs == nil {
		w.refs = newRefIndex()
	}
	names := make([]string, 0, len(w.docs))
	for name := range w.docs {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []refEntry
	for _, name := range names {
		if w.refs.docs[name] == nil {
			w.indexReferencesLocked(w.docs[name])
		}
		out = append(out, w.refs.docs[name][key]...)
	}
	return out
}

// indexReferencesLocked builds doc's reverse-index table, as a query owned by
// doc: the resolutions it memoizes and the table itself go when doc or a
// document it read changes.
func (w *Workspace) indexReferencesLocked(doc *Document) {
	w.refs.docs[doc.Name] = map[symbols.ElementKey][]refEntry{}
	if doc.Scope == nil {
		return
	}
	w.queryLocked(doc.Name, func(r *resolve.Resolver, sem *semantics.Model) {
		for _, ref := range resolve.References(doc.AST, doc.Scope) {
			if ref.QN == nil || len(ref.QN.Parts) == 0 {
				continue
			}
			r.ResolveReference(ref)
			sel := invocationSelection(r, sem, ref)
			elements := segmentElements(r, ref, sel)
			written := segmentNames(r, ref, sel)
			for i := range ref.QN.Parts {
				w.refs.add(doc, ref, i, elements[i], written[i])
			}
		}
	})
}

// ReferencesTo returns every segment in the workspace's documents that reaches
// target or writes its name (an alias use counts for both), in document then position order.
func (w *Workspace) ReferencesTo(target *symbols.Symbol) []ReferenceLocation {
	return w.referenceLocations(target, func(refEntry) bool { return true })
}

// NameReferencesTo returns every segment in the workspace's documents that writes
// name as target's own name — what renaming that name edits. A segment spelling
// target's other name (short for long, or the reverse) still resolves after the
// rename and is left alone; an alias use is edited via the alias.
func (w *Workspace) NameReferencesTo(target *symbols.Symbol, name string) []ReferenceLocation {
	return w.referenceLocations(target, func(e refEntry) bool { return e.named && e.text == name })
}

// referenceLocations answers a reverse-index query for target, building the index
// first when it is stale; the answer is a copy the caller owns.
func (w *Workspace) referenceLocations(target *symbols.Symbol, keep func(refEntry) bool) []ReferenceLocation {
	if target == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	entries := w.referencesLocked(symbols.KeyOf(target))
	out := make([]ReferenceLocation, 0, len(entries))
	for _, e := range entries {
		if keep(e) {
			out = append(out, e.ReferenceLocation)
		}
	}
	return out
}

// RenameConflict reports why renaming target's name (long or short, as written)
// to newName is refused: the name already taken where target is declared, or a
// reference in any workspace document that would read another element afterwards.
func (w *Workspace) RenameConflict(target *symbols.Symbol, name, newName string) *edit.RenameConflict {
	if target == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var occurrences []edit.RenameOccurrence
	for _, e := range w.referencesLocked(symbols.KeyOf(target)) {
		if e.named && e.text == name {
			occurrences = append(occurrences, edit.RenameOccurrence{Ref: e.ref, Part: e.part})
		}
	}
	var conflict *edit.RenameConflict
	w.queryLocked(target.DocName, func(r *resolve.Resolver, sem *semantics.Model) {
		conflict = edit.CheckRename(r, sem, target, name, newName, occurrences)
	})
	return conflict
}

// segmentElements is the element each segment of a resolved ref reaches (nil where
// unresolved); the last is the overload a call selects, nothing when several tie.
func segmentElements(r *resolve.Resolver, ref resolve.Reference, sel *semantics.InvocationSelection) []*symbols.Symbol {
	out := make([]*symbols.Symbol, len(ref.QN.Parts))
	for i := range ref.QN.Parts {
		if sym, ok := r.PartSymbol(ref.QN, i); ok {
			out[i] = sym
		}
	}
	last := len(out) - 1
	out[last] = calledDeclaration(sel, out[last])
	return out
}

// segmentNames is what each segment of a resolved ref writes rather than reaches,
// so a segment written as an alias name is the alias.
func segmentNames(r *resolve.Resolver, ref resolve.Reference, sel *semantics.InvocationSelection) []*symbols.Symbol {
	out := make([]*symbols.Symbol, len(ref.QN.Parts))
	for i := range ref.QN.Parts {
		if sym, ok := r.PartName(ref.QN, i); ok {
			out[i] = sym
		}
	}
	// A written alias stays the name — the alias naming the overload the call selects,
	// when same-named aliases name several — unless the call is tied: then none is.
	last := len(out) - 1
	if _, aliased := r.PartAlias(ref.QN, last); !aliased || (sel != nil && sel.Ambiguous) {
		out[last] = calledDeclaration(sel, out[last])
	} else if element, _ := r.PartSymbol(ref.QN, last); sel != nil && sel.Selected != nil && element != sel.Selected {
		out[last] = selectedName(r, ref, sel.Selected)
	}
	return out
}
