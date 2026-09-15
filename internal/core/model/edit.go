package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/edit"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// EditResult is what ApplyEdit computed: the edited document first, then every
// other document a rename or delete followed a reference into, in name order.
type EditResult struct {
	Documents []DocumentEdit
}

// DocumentEdit is one document's rewrite: the content it was computed from,
// what it becomes, and what each operation changed in it. Version is the one
// the workspace held the content at; Open reports whether that is a client
// buffer's version, or 0 for content read from disk.
type DocumentEdit struct {
	Name     string
	Original []byte
	Content  []byte
	Applied  []edit.Applied
	Version  int
	Open     bool
}

// ApplyEdit rewrites the named document's current content by ops and returns
// the result without changing the workspace: the client owns the buffers, so it
// applies the text and the changes arrive back through the usual document
// notifications. A rename or a cascading delete also rewrites every other
// document of the workspace that refers to the target, open or read from disk;
// every document is read under the one lock, so the edit is computed from a
// single coherent state and each rewrite reports the version it was read at.
// The returned version is the named document's. An unknown document reports ok
// false; a refused edit is an *edit.Error and rewrites nothing.
func (w *Workspace) ApplyEdit(name string, ops []edit.Operation) (result *EditResult, version int, ok bool, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := w.docs[name]
	if doc == nil {
		return nil, 0, false, nil
	}
	ei := w.editIndexLocked(name)
	m := edit.Model{
		Source:     doc.sf,
		Root:       doc.AST,
		Index:      w.index,
		ParseDiags: doc.ParseDiagnostics,
		SemDiags:   w.diagnosticsLocked(name, doc),
		NewIndex:   ei.build,
		Indexed:    ei.indexed,
		Analysis:   w.analysis,
		Other:      w.otherDocumentLocked(name),
	}
	edited, err := edit.Apply(m, ops)
	if err != nil {
		return nil, doc.Version, true, err
	}
	result = &EditResult{Documents: []DocumentEdit{w.documentEditLocked(doc, edited.Content, edited.Applied)}}
	for _, other := range edited.Others {
		result.Documents = append(result.Documents,
			w.documentEditLocked(w.docs[other.Name], other.Content, other.Applied))
	}
	return result, doc.Version, true, nil
}

// documentEditLocked pairs doc as the workspace holds it with its rewrite.
func (w *Workspace) documentEditLocked(doc *Document, content []byte, applied []edit.Applied) DocumentEdit {
	return DocumentEdit{
		Name:     doc.Name,
		Original: doc.Content,
		Content:  content,
		Applied:  applied,
		Version:  doc.Version,
		Open:     w.open[doc.Name],
	}
}

// otherDocumentLocked hands an edit of name the other documents of the
// workspace as it holds them, so a rename or delete may rewrite them. Library
// documents are not workspace documents and are not handed out.
func (w *Workspace) otherDocumentLocked(name string) func(string) (edit.Document, bool) {
	return func(other string) (edit.Document, bool) {
		doc := w.docs[other]
		if other == name || doc == nil {
			return edit.Document{}, false
		}
		return edit.Document{
			Source:     doc.sf,
			ParseDiags: doc.ParseDiagnostics,
			SemDiags:   w.diagnosticsLocked(other, doc),
		}, true
	}
}

// editIndex is the index one edit of the named document is judged in, built once
// and re-fed each rewrite; standIns is what its documents stand in for, which an
// edit's intermediate rewrites move without moving the workspace's own.
type editIndex struct {
	w        *Workspace
	name     string
	standIns map[string]string
}

// editIndexLocked prepares the index an edit of name is judged in. Caller holds the lock.
func (w *Workspace) editIndexLocked(name string) *editIndex {
	return &editIndex{w: w, name: name}
}

// build makes an index holding the libraries and every workspace document but
// the edited one, so the edited notation resolves what the original did. Over a
// shared library base it overlays that; over a caller-built index it re-indexes
// the caller's library files, marked, and its other documents, languages
// included. A bundled file the edited document stands in for stays: indexed
// displaces it again if the edited notation is still a version of it.
func (e *editIndex) build() *symbols.Index {
	w, name := e.w, e.name
	var idx *symbols.Index
	if w.libBase != nil {
		idx = symbols.NewOverlay(w.libBase)
	} else {
		idx = symbols.NewIndex()
		for other, file := range w.library {
			if other == name || w.docs[other] != nil {
				continue
			}
			idx.AddDocumentWithKind(other, file.root, file.kind)
			idx.MarkLibraryDocument(other, file.record)
		}
		for _, other := range w.index.Documents() {
			if _, library := w.library[other]; library || other == name || w.docs[other] != nil {
				continue
			}
			root, ok := w.index.DocumentRoot(other).Node().(*ast.RootNamespace)
			if !ok {
				continue
			}
			idx.AddDocumentWithKind(other, root, w.index.DocumentKind(other))
		}
	}
	e.standIns = map[string]string{}
	for other, library := range w.standIns {
		if other != name {
			e.standIns[other] = library
			idx.RemoveDocument(library)
		}
	}
	for other, doc := range w.docs {
		if other == name {
			continue
		}
		idx.AddDocument(other, doc.AST)
		if _, ok := w.standIns[other]; ok {
			idx.MarkLibraryDocument(other, w.index.LibraryDocumentOf(other))
		}
	}
	idx.ExpandWildcardImports()
	return idx
}
