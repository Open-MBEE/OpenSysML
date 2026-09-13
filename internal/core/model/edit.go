package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/edit"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ApplyEdit rewrites the named document's current content by ops and returns
// the result without changing the workspace: the client owns the buffer, so it
// applies the text and the change arrives back through the usual document
// notifications. The returned version is the one the content was read at. An
// unknown document reports ok false; a refused edit is an *edit.Error.
func (w *Workspace) ApplyEdit(name string, ops []edit.Operation) (result *edit.Result, version int, ok bool, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := w.docs[name]
	if doc == nil {
		return nil, 0, false, nil
	}
	m := edit.Model{
		Source:     doc.sf,
		Root:       doc.AST,
		Index:      w.index,
		ParseDiags: doc.ParseDiagnostics,
		SemDiags:   w.diagnosticsLocked(name, doc),
		NewIndex:   w.siblingIndexLocked(name),
		Analysis:   w.analysis,
	}
	result, err = edit.Apply(m, ops)
	return result, doc.Version, true, err
}

// siblingIndexLocked builds an index holding the libraries and every workspace
// document but name, so the edited notation resolves what the original did.
// Nil when the workspace's index has no library base to overlay.
func (w *Workspace) siblingIndexLocked(name string) func() *symbols.Index {
	if w.libBase == nil {
		return nil
	}
	return func() *symbols.Index {
		idx := symbols.NewOverlay(w.libBase)
		for other, doc := range w.docs {
			if other != name {
				idx.AddDocument(other, doc.AST)
			}
		}
		idx.ExpandWildcardImports()
		return idx
	}
}
