package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A keyword that takes its prefix metadata after itself (`subject #M s;`)
// opens the declaration: the span stays on it even when the metadata name
// spells the keyword, with and without the source text.
func TestDeclarationKeywordSpanKeepsKeywordBeforeItsPrefixMetadata(t *testing.T) {
	const name, src = "a.kerml", "subject #'subject' s;"
	pm := &ast.PrefixMetadata{}
	pm.NodeSpan = source.Span{Offset: strings.Index(src, "#"), Len: len("#'subject'")}
	u := &ast.Usage{Keyword: "subject", Prefixes: []*ast.PrefixMetadata{pm}}
	u.NodeSpan = source.Span{Offset: 0, Len: len(src)}
	want := source.Span{Offset: 0, Len: len("subject")}

	sf := source.New(name, []byte(src))
	w := &notationWalker{doc: name, lookup: source.TextOf(map[string]*source.SourceFile{name: sf}, nil)}
	if got := w.declarationKeywordSpan(u, "subject"); got != want {
		t.Errorf("with source text: span = %+v, want %+v", got, want)
	}
	w.lookup = nil
	if got := w.declarationKeywordSpan(u, "subject"); got != want {
		t.Errorf("without source text: span = %+v, want %+v", got, want)
	}
}
