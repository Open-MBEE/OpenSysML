package model

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Hydrate parses the named recorded document and holds it loaded: its record
// symbols are replaced by tree-backed ones, and its dependents are invalidated
// as by an edit. A loaded document is left as it is; ErrNoDocument names one
// the workspace does not hold.
func (w *Workspace) Hydrate(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := w.docs[name]
	if doc == nil {
		return fmt.Errorf("%w: %s", ErrNoDocument, name)
	}
	if !doc.Recorded() {
		return nil
	}
	w.hydrateLocked(name)
	w.index.ExpandWildcardImports()
	w.invalidateLocked(name)
	return nil
}

// Recorded reports whether the named document is held as its interface record.
func (w *Workspace) Recorded(name string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.docs[name].Recorded()
}

// hydrateLocked parses the recorded document's text and installs the tree-backed
// document in its place; the caller expands wildcard imports and invalidates.
// Caller holds the write lock.
func (w *Workspace) hydrateLocked(name string) *Document {
	held := w.docs[name]
	doc := newDocument(name, held.Content, held.Version)
	w.docs[name] = doc
	w.changes[name]++
	w.installLocked(doc)
	return doc
}

// HydrateAll hydrates every document held as its record, for a consumer that
// reads the trees of them all; dependents are invalidated as by an edit.
func (w *Workspace) HydrateAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.hydrateAllLocked()
}

// hydrateAllLocked hydrates every document held as its record, for a question
// over the bodies of them all: the references written in the workspace, a
// runtime over it. Caller holds the write lock.
func (w *Workspace) hydrateAllLocked() {
	var recorded []string
	for name, doc := range w.docs {
		if doc.Recorded() {
			recorded = append(recorded, name)
		}
	}
	if len(recorded) == 0 {
		return
	}
	slices.Sort(recorded)
	parsed := make([]*Document, len(recorded))
	ParallelFor(w.workers, len(recorded), func(i int) {
		held := w.docs[recorded[i]]
		parsed[i] = newDocument(held.Name, held.Content, held.Version)
	})
	for i, name := range recorded {
		w.docs[name] = parsed[i]
		w.changes[name]++
		w.installLocked(parsed[i])
	}
	w.index.ExpandWildcardImports()
	w.invalidateLocked(recorded...)
}

// hydrateStaleLocked hydrates every recorded document whose recording analysis
// read something ch moved and is no longer answered as it was: the record's
// diagnostics and facts followed from what was read, as a live frame's memo
// does, and go the same way. A read the same documents, unchanged, still answer
// — one moved by a hydration, say — leaves the record holding. The hydrations
// are a change of their own, invalidated in turn. Caller holds the write lock.
func (w *Workspace) hydrateStaleLocked(ch symbols.Changes) {
	var stale []string
	var src *libs.Sources
	for name, doc := range w.docs {
		if !doc.Recorded() || ch.Docs[name] || !doc.recorded.provenance.Reads.Stale(ch) {
			continue
		}
		if src == nil {
			sources := w.sourcesLocked()
			src = &sources
		}
		if !doc.recorded.provenance.Valid(*src) {
			stale = append(stale, name)
		}
	}
	if len(stale) == 0 {
		return
	}
	slices.Sort(stale)
	for _, name := range stale {
		w.hydrateLocked(name)
	}
	w.index.ExpandWildcardImports()
	w.invalidateLocked(stale...)
}

// demoteLocked holds the loaded document as its interface record, releasing its
// tree, when a record of its content is to be had: the record cache's, where
// its provenance holds here, else one written from the analysis the workspace
// holds. It reports false when the document stays loaded: it stands in for a
// library file, has not been analyzed, or is unrecordable. Caller holds the
// write lock.
func (w *Workspace) demoteLocked(name string, doc *Document) bool {
	if doc.Recorded() {
		return true
	}
	if w.standIns[name] != "" {
		return false
	}
	rec := w.cachedRecordLocked(name, doc.Content)
	if rec == nil {
		if _, analyzed := w.diagCache[name]; !analyzed {
			return false
		}
		var err error
		if rec, err = w.interfaceRecordLocked(name, doc, w.sourcesLocked()); err != nil {
			return false
		}
		w.storeRecordLocked(rec, doc.Content)
	}
	scope, err := symbols.BuildRecorded(rec.Scope, rec.Name)
	if err != nil {
		return false
	}
	w.installRecordedLocked(rec, scope, doc.Content, doc.Version)
	w.index.ExpandWildcardImports()
	w.invalidateLocked(name)
	return true
}

// holdOnDiskLocked holds the closed document name with the content its file
// has: as the record cache's record of that content, where the workspace has
// one whose provenance holds, else parsed. Caller holds the write lock.
func (w *Workspace) holdOnDiskLocked(name string, content []byte) {
	if rec := w.cachedRecordLocked(name, content); rec != nil {
		if scope, err := symbols.BuildRecorded(rec.Scope, rec.Name); err == nil {
			w.installRecordedLocked(rec, scope, content, 0)
			w.index.ExpandWildcardImports()
			w.invalidateLocked(name)
			return
		}
	}
	w.reindexLocked(name, content, 0)
}

// cachedRecordLocked is the record cache's record of the named content, when
// the workspace has a cache, the record answers its question and its
// provenance holds among the documents held. Caller holds the write lock.
func (w *Workspace) cachedRecordLocked(name string, content []byte) *libs.InterfaceRecord {
	if w.records == nil {
		return nil
	}
	rec, ok := w.records.LoadInterface(w.recordKeyLocked(name, content))
	if !ok || w.recordAcceptedLocked(rec) != nil || !rec.Provenance.Valid(w.sourcesLocked()) {
		return nil
	}
	return rec
}

// storeRecordLocked writes rec to the record cache, when the workspace has one.
// A cache that cannot be written is no cache: the record is held in memory only.
func (w *Workspace) storeRecordLocked(rec *libs.InterfaceRecord, content []byte) {
	if w.records == nil {
		return
	}
	_ = w.records.StoreInterface(w.recordKeyLocked(rec.Name, content), rec)
}

// recordKeyLocked is the record cache's key for the named content in this
// workspace: its library and conformance mode are the key's.
func (w *Workspace) recordKeyLocked(name string, content []byte) string {
	if w.libDigest == "" {
		w.libDigest = libs.SourceDigest(w.libSource)
	}
	return w.records.InterfaceKey(name, content, w.libDigest, w.analysis.Conformance)
}

// WriteRecord writes the named loaded document's interface record to the
// record cache, analyzing it first if it has not been: for a document just
// saved, so a later workspace holds it as its record. Without a cache, or for
// a recorded document, it does nothing; an unrecordable document is an error.
func (w *Workspace) WriteRecord(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := w.docs[name]
	if doc == nil {
		return fmt.Errorf("%w: %s", ErrNoDocument, name)
	}
	if w.records == nil || doc.Recorded() {
		return nil
	}
	rec, err := w.interfaceRecordLocked(name, doc, w.sourcesLocked())
	if err != nil {
		return err
	}
	w.storeRecordLocked(rec, doc.Content)
	return nil
}

// writeRecordsLocked writes the records of the named loaded documents batch
// just analyzed to the record cache, when the workspace has one; a document
// that cannot be recorded is left out. Provenance is attributed over the
// gathers the batch's workers shared, which already hold the unions the
// analyses read. Caller holds the write lock.
func (w *Workspace) writeRecordsLocked(names []string, batch *passes.Batch) {
	if w.records == nil {
		return
	}
	src := w.sourcesOverLocked(w.batchContextLocked(batch))
	for _, name := range names {
		doc := w.docs[name]
		if doc == nil || doc.Recorded() {
			continue
		}
		if rec, err := w.interfaceRecordLocked(name, doc, src); err == nil {
			w.storeRecordLocked(rec, doc.Content)
		}
	}
}
