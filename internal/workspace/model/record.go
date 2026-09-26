package model

import (
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
	return libs.WriteInterface(name, source.KindOf(name), w.index, resolver, sem, gathered, diags)
}

// OpenRecorded installs a document from its interface record, in place of any
// document of its name: tree-less scopes and symbols carrying the record's
// facts, reporting the diagnostics stored with it. Dependents are invalidated
// as by any replacement. The workspace keeps what it built, not rec: a
// recorded document costs its scopes, symbols and diagnostics.
func (w *Workspace) OpenRecorded(rec *libs.InterfaceRecord) error {
	scope, err := symbols.BuildRecorded(rec.Scope, rec.Name)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := &Document{Name: rec.Name, Scope: scope, recorded: &recordedDiagnostics{diagnostics: rec.Diagnostics}}
	w.docs[rec.Name] = doc
	w.changes[rec.Name]++
	w.displaceLocked(rec.Name)
	w.releaseStandInLocked(rec.Name)
	w.index.AddRecordedDocument(rec.Name, rec.Kind, scope, rec.Scope.Gathered)
	w.index.ExpandWildcardImports()
	w.invalidateLocked(rec.Name)
	return nil
}
