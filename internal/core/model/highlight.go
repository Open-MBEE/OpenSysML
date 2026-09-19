package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/highlight"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// HighlightTokens returns the semantic tokens of a document, ordered by source
// position, with the content they index; the workspace or bundled library
// document is chosen under one lock, so neither an edit nor a close can split
// or lose them. Nil for unknown documents.
func (w *Workspace) HighlightTokens(name string) ([]byte, []highlight.Token) {
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := w.docs[name]
	if doc == nil {
		if lib, isLibrary := w.libraryNameLocked(name); isLibrary {
			doc = w.libraryDocumentLocked(lib)
		}
	}
	if doc == nil {
		return nil, nil
	}
	var out []highlight.Token
	w.queryLocked(name, func(resolver *resolve.Resolver, _ *semantics.Model) {
		out = highlight.Tokens(doc.Content, doc.AST, doc.Scope, resolution{r: resolver})
	})
	return doc.Content, out
}

// resolution answers highlighting queries from one resolver, so the memoized
// resolution of a name is shared across the document's references.
type resolution struct{ r *resolve.Resolver }

func (res resolution) SegmentSymbols(ref resolve.Reference) []*symbols.Symbol {
	if ref.QN == nil {
		return nil
	}
	res.r.ResolveReference(ref)
	out := make([]*symbols.Symbol, len(ref.QN.Parts))
	for i := range ref.QN.Parts {
		if sym, ok := res.r.PartSymbol(ref.QN, i); ok {
			out[i] = sym
		}
	}
	return out
}
