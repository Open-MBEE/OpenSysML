package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/highlight"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// HighlightTokens returns the semantic tokens of a document, ordered by source
// position, with the content they index, so an edit cannot split the two. Nil
// for unknown documents. A bundled library document is highlighted like a
// workspace one.
func (w *Workspace) HighlightTokens(name string) ([]byte, []highlight.Token) {
	lib := w.LibraryDocument(name)
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := w.docs[name]
	if doc == nil {
		doc = lib
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
