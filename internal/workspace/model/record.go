package model

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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

// InterfaceRecord writes the interface record of the named loaded document from
// the analysis of it this workspace holds, analyzing it first if it has not
// been (see libs.WriteInterface). A document that stands in for a library file
// is not recordable: what it stands in for is read from its tree. A document
// already held as its record reports ErrRecorded: its record is in the cache it
// was read from. The relationships the workspace-wide audits gather from the
// body ride along by reference (see passes.GatherRelationships).
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
	if w.standIns[name] != "" {
		return nil, fmt.Errorf("%w: %s stands in for library file %s", libs.ErrUnrecordable, name, w.standIns[name])
	}
	diags := w.diagnosticsLocked(name, doc)
	gathered, err := passes.GatherRelationships(w.contextLocked(), name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", libs.ErrUnrecordable, err)
	}
	resolver, sem := w.semanticsLocked()
	rec, err := libs.WriteInterface(name, source.KindOf(name), w.index, resolver, sem, gathered, diags)
	if err != nil {
		return nil, err
	}
	rec.Digest = doc.digest
	rec.Mode = w.analysis.Conformance
	return rec, nil
}

// OpenRecorded installs a document from its interface record and the content
// the record was written from, in place of any document of its name: tree-less
// scopes and symbols carrying the record's facts, reporting the diagnostics
// stored with it against content, whose positions they hold. Content is
// checked against the record's digest, ErrRecordMismatch when it differs.
// Dependents are invalidated as by any replacement. The workspace keeps what
// it built, not rec: a recorded document costs its text, scopes, symbols and
// diagnostics.
func (w *Workspace) OpenRecorded(rec *libs.InterfaceRecord, content []byte) error {
	content = bytes.Clone(content)
	digest := digestOf(content)
	if digest != rec.Digest {
		return fmt.Errorf("%w: %s", ErrRecordMismatch, rec.Name)
	}
	scope, err := symbols.BuildRecorded(rec.Scope, rec.Name)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if rec.Mode != w.analysis.Conformance {
		return fmt.Errorf("%w: %s was recorded under conformance mode %s, the workspace asks %s", ErrRecordMismatch, rec.Name, rec.Mode, w.analysis.Conformance)
	}
	doc := &Document{
		Name:     rec.Name,
		Content:  content,
		Scope:    scope,
		sf:       source.New(rec.Name, content),
		digest:   digest,
		recorded: &recordedDiagnostics{diagnostics: rec.Diagnostics},
	}
	w.docs[rec.Name] = doc
	w.changes[rec.Name]++
	w.displaceLocked(rec.Name)
	w.releaseStandInLocked(rec.Name)
	w.index.AddRecordedDocument(rec.Name, rec.Kind, scope, rec.Scope.Gathered)
	w.index.ExpandWildcardImports()
	w.invalidateLocked(rec.Name)
	return nil
}
