package model

import (
	"fmt"

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
	m := edit.Model{
		Source:     doc.sf,
		Root:       doc.AST,
		Index:      w.index,
		ParseDiags: doc.ParseDiagnostics,
		SemDiags:   w.diagnosticsLocked(name, doc),
		NewIndex:   w.siblingIndexLocked(name),
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

// siblingIndexLocked builds an index holding the libraries and every workspace
// document but name, so the edited notation resolves what the original did.
func (w *Workspace) siblingIndexLocked(name string) func() *symbols.Index {
	return func() *symbols.Index {
		idx := w.detachedIndexLocked()
		for other, doc := range w.docs {
			if other != name {
				idx.AddDocument(other, doc.AST)
			}
		}
		idx.ExpandWildcardImports()
		return idx
	}
}

// detachedIndexLocked is a writable index holding what the workspace's index
// holds besides the workspace's documents: over its frozen base when it has one,
// with the caller's other documents re-indexed, library marks and languages included.
func (w *Workspace) detachedIndexLocked() *symbols.Index {
	base := w.index.Base()
	var idx *symbols.Index
	if base != nil {
		idx = symbols.NewOverlay(base)
	} else {
		idx = symbols.NewIndex()
	}
	for _, other := range w.index.Documents() {
		if w.docs[other] != nil {
			continue
		}
		scope := w.index.DocumentRoot(other)
		if base != nil && base.DocumentRoot(other) == scope {
			continue
		}
		root, ok := scope.Node().(*ast.RootNamespace)
		if !ok {
			continue
		}
		idx.AddDocumentWithKind(other, root, w.index.DocumentKind(other))
		if lib := w.index.LibraryDocumentOf(other); lib.Tier.Library() {
			idx.MarkLibraryDocument(other, lib)
		}
	}
	return idx
}
