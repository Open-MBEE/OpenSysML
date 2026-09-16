package model

import (
	"fmt"
	"maps"
	"slices"

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

// StaleError reports that a document an operation was computed against has
// been replaced since, so a position in it may name something else now.
type StaleError struct {
	Name string
}

func (e *StaleError) Error() string {
	return fmt.Sprintf("%s changed after the operation was computed against it", e.Name)
}

// ApplyEdit rewrites the named document's current content by ops and returns
// the result without changing the workspace: the client owns the buffers, so it
// applies the text and the changes arrive back through the usual document
// notifications. A rename or a cascading delete also rewrites every other
// document of the workspace that refers to the target, open or read from disk;
// every document is read under the one lock, so the edit is computed from a
// single coherent state and each rewrite reports the version it was read at.
// Read are the workspace documents, as handed out earlier, that the operations'
// positions were computed from; when the workspace no longer holds one of those
// snapshots, the edit is refused with a *StaleError. The returned version is the
// named document's. An unknown document reports ok false; a refused edit is an
// *edit.Error and rewrites nothing.
func (w *Workspace) ApplyEdit(name string, ops []edit.Operation, read []*Document) (result *EditResult, version int, ok bool, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := w.docs[name]
	if doc == nil {
		return nil, 0, false, nil
	}
	for _, snapshot := range read {
		if w.docs[snapshot.Name] != snapshot {
			return nil, doc.Version, true, &StaleError{Name: snapshot.Name}
		}
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
		Documents:  w.otherDocumentNamesLocked(name),
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

// otherDocumentNamesLocked names the documents other than name an edit reads
// references in: the index's unmarked ones and the workspace's own, a version
// standing in for a library file included.
func (w *Workspace) otherDocumentNamesLocked(name string) []string {
	names := map[string]bool{}
	for _, other := range w.index.WorkspaceDocuments() {
		names[other] = true
	}
	for other := range w.docs {
		names[other] = true
	}
	delete(names, name)
	return slices.Sorted(maps.Keys(names))
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

// build makes an index holding the libraries and every workspace document but the
// edited one; a bundled file it stands in for stays until indexed displaces it again.
func (e *editIndex) build() *symbols.Index {
	idx := e.w.detachedIndexLocked()
	e.standIns = e.w.addDocumentsLocked(idx, e.name)
	idx.ExpandWildcardImports()
	return idx
}

// detachedIndexLocked is a writable index holding what the workspace's index holds
// besides its documents: the base less what the overlay removed, libraries marked.
func (w *Workspace) detachedIndexLocked() *symbols.Index {
	idx := symbols.NewIndex()
	if w.libBase != nil {
		idx = symbols.NewOverlay(w.libBase)
		for _, other := range w.libBase.Documents() {
			if _, library := w.library[other]; !library && w.index.DocumentRoot(other) == nil {
				idx.RemoveDocument(other)
			}
		}
	}
	added := map[string]bool{}
	skip := func(other string) bool {
		return w.docs[other] != nil || added[other] || w.baseShows(other)
	}
	libraries := make([]string, 0, len(w.library))
	for other := range w.library {
		libraries = append(libraries, other)
	}
	slices.Sort(libraries)
	for _, other := range libraries {
		if skip(other) {
			continue
		}
		file := w.library[other]
		idx.AddDocumentWithKind(other, file.root, file.kind)
		idx.MarkLibraryDocument(other, file.record)
		added[other] = true
	}
	for _, other := range w.index.Documents() {
		if skip(other) {
			continue
		}
		root, ok := w.index.DocumentRoot(other).Node().(*ast.RootNamespace)
		if !ok {
			continue
		}
		idx.AddDocumentWithKind(other, root, w.index.DocumentKind(other))
	}
	return idx
}

// addDocumentsLocked indexes the workspace's documents but except on idx, each one
// standing in for a bundled file displacing it, and reports what stands in for what.
func (w *Workspace) addDocumentsLocked(idx *symbols.Index, except string) map[string]string {
	standIns := map[string]string{}
	for other, library := range w.standIns {
		if other != except {
			standIns[other] = library
			idx.RemoveDocument(library)
		}
	}
	for _, other := range w.sortedDocNamesLocked() {
		if other == except {
			continue
		}
		idx.AddDocumentWithKind(other, w.docs[other].AST, w.index.DocumentKind(other))
		if _, ok := standIns[other]; ok {
			idx.MarkLibraryDocument(other, w.index.LibraryDocumentOf(other))
		}
	}
	return standIns
}
