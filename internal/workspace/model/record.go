package model

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// ErrNoDocument reports a name the workspace holds no document under.
var ErrNoDocument = errors.New("workspace: no such document")

// ErrRecorded reports a document held as its interface record where its tree
// is asked for: the workspace does not keep the record it was installed from.
var ErrRecorded = errors.New("workspace: the document is held as its interface record")

// ErrRecordMismatch reports content offered with an interface record that was
// written from other bytes: its diagnostics and spans would not be the content's.
var ErrRecordMismatch = errors.New("workspace: content does not match its interface record")

// ErrRecordStale reports an interface record installed among documents other
// than the ones its analysis read: its diagnostics and evaluated facts would
// not follow from them, so the document was parsed instead and is held loaded.
var ErrRecordStale = errors.New("workspace: the interface record was written among other documents")

// InterfaceRecord writes the interface record of the named loaded document from
// the analysis of it this workspace holds, analyzing it first if it has not
// been (see libs.WriteInterface). A document that stands in for a library file
// is not recordable: what it stands in for is read from its tree. A document
// already held as its record reports ErrRecorded: its record is in the cache it
// was read from. The relationships the workspace-wide audits gather from the
// body ride along by reference (see passes.GatherRelationships), and so does
// where the analysis got its answers (see libs.Provenance).
func (w *Workspace) InterfaceRecord(name string) (*libs.InterfaceRecord, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := w.docs[name]
	if doc == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoDocument, name)
	}
	if doc.Recorded() {
		return nil, fmt.Errorf("%w: %s", ErrRecorded, name)
	}
	return w.interfaceRecordLocked(name, doc, w.sourcesLocked())
}

// interfaceRecordLocked is InterfaceRecord for a loaded document, attributing
// its provenance against src, which a caller writing many records shares.
// Caller holds the write lock.
func (w *Workspace) interfaceRecordLocked(name string, doc *Document, src libs.Sources) (*libs.InterfaceRecord, error) {
	if w.standIns[name] != "" {
		return nil, fmt.Errorf("%w: %s stands in for library file %s", libs.ErrUnrecordable, name, w.standIns[name])
	}
	if reads, ok := w.batched[name]; ok && reads == nil {
		// Analyzed by a batch that recorded nothing: analyze again, recording.
		delete(w.batched, name)
		delete(w.diagCache, name)
	}
	diags := w.diagnosticsLocked(name, doc)
	gathered, err := passes.GatherRelationships(w.contextLocked(), name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", libs.ErrUnrecordable, err)
	}
	resolver, sem := w.semanticsLocked()
	reads, ok := w.readsLocked(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s has no analysis to record", libs.ErrUnrecordable, name)
	}
	rec, err := libs.WriteInterface(name, doc.Kind(), w.index, resolver, sem, gathered, diags)
	if err != nil {
		return nil, err
	}
	if rec.Provenance, err = libs.Attribute(reads, src); err != nil {
		return nil, err
	}
	rec.Digest = doc.digest
	rec.Mode = w.analysis.Conformance
	rec.Members = libs.TopMembers(doc.AST)
	return rec, nil
}

// readsLocked is what the analysis whose diagnostics the workspace holds for
// name read: the shared resolver's frame when it was analyzed there, else the
// batch's. Caller holds the write lock.
func (w *Workspace) readsLocked(name string) (resolve.Reads, bool) {
	if reads := w.batched[name]; reads != nil {
		return *reads, true
	}
	return w.resolver.ReadsOf(name)
}

// sourcesLocked is where the index's answers come from, for attributing or
// checking a record's provenance; it memoizes them, so take a fresh one after
// the index changes. Caller holds the write lock.
func (w *Workspace) sourcesLocked() libs.Sources {
	return w.sourcesOverLocked(w.contextLocked())
}

// sourcesOverLocked is sourcesLocked answering about shared state through ctx:
// a batch's, whose workers built the unions its records read. Caller holds the
// write lock.
func (w *Workspace) sourcesOverLocked(ctx *passes.Context) libs.Sources {
	return libs.NewSources(w.index,
		func(name string) ([]string, bool) { return passes.Contributors(ctx, name) },
		func(name string) (string, bool) {
			doc := w.docs[name]
			if doc == nil {
				return "", false
			}
			return doc.digest, true
		},
		func(name string) string { return w.standIns[name] })
}

// OpenRecorded installs a document from its interface record and the content
// the record was written from, in place of any document of its name: tree-less
// scopes and symbols carrying the record's facts, reporting the diagnostics
// stored with it against content, whose positions they hold. Content is
// checked against the record's digest, ErrRecordMismatch when it differs.
// Dependents are invalidated as by any replacement. A recorded document is a
// closed file: any open buffer of its name is dropped, and content is what the
// file holds on disk, so a later change to the file reindexes it. The workspace
// keeps what it built, not rec: a recorded document costs its text, scopes,
// symbols and diagnostics.
//
// The record holds only among the documents its analysis read as they were
// (see libs.Provenance): where the workspace holds others, the document is
// parsed in the record's place, held loaded, and ErrRecordStale reported.
func (w *Workspace) OpenRecorded(rec *libs.InterfaceRecord, content []byte) error {
	content = bytes.Clone(content)
	if digestOf(content) != rec.Digest {
		return fmt.Errorf("%w: %s", ErrRecordMismatch, rec.Name)
	}
	scope, err := symbols.BuildRecorded(rec.Scope, rec.Name)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.recordAcceptedLocked(rec); err != nil {
		return err
	}
	w.installRecordedLocked(rec, scope, content, 0)
	w.index.ExpandWildcardImports()
	w.invalidateLocked(rec.Name)
	if !w.docs[rec.Name].Recorded() || rec.Provenance.Valid(w.sourcesLocked()) {
		return nil
	}
	w.hydrateLocked(rec.Name)
	w.index.ExpandWildcardImports()
	w.invalidateLocked(rec.Name)
	return fmt.Errorf("%w: %s", ErrRecordStale, rec.Name)
}

// recordAcceptedLocked reports whether rec answers this workspace's question:
// ErrRecordMismatch when it was written under another conformance mode.
func (w *Workspace) recordAcceptedLocked(rec *libs.InterfaceRecord) error {
	if rec.Mode != w.analysis.Conformance {
		return fmt.Errorf("%w: %s was recorded under conformance mode %s, the workspace asks %s", ErrRecordMismatch, rec.Name, rec.Mode, w.analysis.Conformance)
	}
	return nil
}

// installRecordedLocked puts the recorded document built from rec over scope in
// place of any document of its name, under version; the caller expands wildcard
// imports and invalidates. The resolver is made first, so the change log the
// record's staleness is judged by is on. Caller holds the write lock.
func (w *Workspace) installRecordedLocked(rec *libs.InterfaceRecord, scope *symbols.Scope, content []byte, version int) {
	w.semanticsLocked()
	sf := newDocumentSource(rec.Name, content, rec.Kind)
	doc := &Document{
		Name:    rec.Name,
		Content: content,
		Version: version,
		Scope:   scope,
		kind:    sf.Kind(),
		sf:      sf,
		digest:  rec.Digest,
		recorded: &recordedDiagnostics{
			diagnostics: diag.Clone(rec.Diagnostics),
			provenance:  rec.Provenance.Clone(),
			members:     slices.Clone(rec.Members),
		},
	}
	w.docs[rec.Name] = doc
	delete(w.open, rec.Name)
	w.onDisk[rec.Name] = content
	w.changes[rec.Name]++
	w.displaceLocked(rec.Name)
	w.releaseStandInLocked(rec.Name)
	w.index.AddRecordedDocument(rec.Name, doc.Kind(), scope, rec.Scope.Gathered.Clone())
}
