package model

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// Reading answers questions of the workspace's documents as they all stand at
// one moment: it lives for one call of Workspace.Read, under the lock.
type Reading struct {
	w *Workspace
}

// Read calls fn with a Reading under the write lock (its answers query the shared
// resolver), so all are of the same documents. fn must not call the workspace itself.
func (w *Workspace) Read(fn func(r *Reading) error) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return fn(&Reading{w: w})
}

// Generation counts the changes made to the workspace's documents so far; two
// readings with the same generation read the same documents.
func (w *Workspace) Generation() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.generation
}

// Generation is the workspace's generation as read.
func (r *Reading) Generation() uint64 { return r.w.generation }

// Document is the held document name, nil for one the workspace does not hold.
func (r *Reading) Document(name string) *Document { return r.w.docs[name] }

// Declared is the element fqn names in doc: by qualified name in the index, else
// by qualified or simple name among the document's own declarations.
func (r *Reading) Declared(doc, fqn string) *symbols.Symbol {
	return declaredIn(r.w.index, doc, fqn)
}

// DeclarationText is the source text of sym's declaration, "" when its document
// is gone.
func (r *Reading) DeclarationText(sym *symbols.Symbol) string {
	if sym == nil || sym.Decl == nil {
		return ""
	}
	return declarationText(r.TextAt, sym.DocName, sym.Decl.Span())
}

// declarationText is the text at span less the trivia after its last token,
// which a declaration's span runs through: an edit after it is not an edit of it.
func declarationText(text func(doc string, span source.Span) string, doc string, span source.Span) string {
	return strings.TrimRight(text(doc, span), " \t\r\n")
}

// TextAt is the source text at span in doc, from the document or the library
// file behind the documents; "" when neither holds doc.
func (r *Reading) TextAt(doc string, span source.Span) string {
	return r.w.sourceText()(doc, span)
}

// DeclaredView is the view RenderView renders for fqn in doc: the view named, the
// document's one view for "", nil for a pseudo-view or a name declaring none.
func (r *Reading) DeclaredView(doc, fqn string) *symbols.Symbol {
	if strings.HasPrefix(fqn, view.PseudoViewPrefix) {
		return nil
	}
	if fqn == "" {
		if views := r.w.documentViewsLocked(doc); len(views) == 1 {
			return views[0]
		}
		return nil
	}
	return r.w.viewNamedLocked(doc, fqn)
}

// RenderView is Workspace.RenderView of the documents as read.
func (r *Reading) RenderView(doc, fqn string) (*view.Rendering, *Document, error) {
	return r.w.renderViewLocked(doc, fqn)
}

// NewRuntime is Workspace.NewRuntime over the documents as read.
func (r *Reading) NewRuntime() (*Runtime, error) {
	return r.w.newRuntimeLocked()
}
