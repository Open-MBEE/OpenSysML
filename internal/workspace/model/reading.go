package model

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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

// Index is the workspace's index as read.
func (r *Reading) Index() *symbols.Index { return r.w.index }

// Query runs fn with the shared semantics, as a query of doc reads them.
func (r *Reading) Query(doc string, fn func(*resolve.Resolver, *semantics.Model)) {
	r.w.queryLocked(doc, fn)
}

// SourceText is the text of the documents and the library files behind them.
func (r *Reading) SourceText() view.SourceText { return r.w.sourceText() }

// Document is the held document name, nil for one the workspace does not hold.
func (r *Reading) Document(name string) *Document { return r.w.docs[name] }

// LineIndex is the held document's line index, nil when the workspace does not hold it.
func (r *Reading) LineIndex(name string) *source.LineIndex {
	if doc := r.Document(name); doc != nil {
		return doc.Lines()
	}
	return nil
}

// LineIndexes snapshots the line indexes of the held documents.
func (r *Reading) LineIndexes() map[string]*source.LineIndex {
	indexes := make(map[string]*source.LineIndex, len(r.w.docs))
	for name, doc := range r.w.docs {
		indexes[name] = doc.Lines()
	}
	return indexes
}

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

// declarationText is the text at span cut after its last visible token: a
// declaration's span runs on through the trivia after it, which is not its own.
func declarationText(text func(doc string, span source.Span) string, doc string, span source.Span) string {
	return cutTrailingTrivia(text(doc, span))
}

// cutTrailingTrivia is text up to the end of its last token the parser reads:
// whitespace, notes and bare /* */ comments are what it skips between tokens.
func cutTrailingTrivia(text string) string {
	lx := lexer.New(source.New("declaration.sysml", []byte(text)))
	end := 0
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if !tok.IsTrivia() && tok.Kind != lexer.RegularComment {
			end = tok.Span.End()
		}
	}
	return text[:end]
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
func (r *Reading) RenderView(doc, fqn string) (*view.Rendering, *Snapshot, error) {
	return r.w.renderViewLocked(doc, fqn, "", nil)
}

// RenderOverlaidView is RenderView with overlay drawn over the rendering, its
// verdicts answered by verdicts.
func (r *Reading) RenderOverlaidView(doc, fqn string, overlay view.Overlay, verdicts view.Verdicts) (*view.Rendering, *Snapshot, error) {
	return r.w.renderViewLocked(doc, fqn, overlay, verdicts)
}

// LinkSites is RenderViewLinked's source sites for a rendering made by this
// reading, frozen into its snapshot.
func (r *Reading) LinkSites(rendering *view.Rendering, snapshot *Snapshot) {
	r.w.linkSitesLocked(rendering, snapshot)
}

// Detach is Workspace.Detach over the documents as read.
func (r *Reading) Detach() (*Detached, error) {
	return r.w.detachLocked()
}
