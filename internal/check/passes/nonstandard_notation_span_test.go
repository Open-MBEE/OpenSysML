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

// A comment ahead of the kind keyword may spell it; the span is the keyword
// token, in either direction.
func TestDeclarationKeywordSpanSkipsComments(t *testing.T) {
	for _, tc := range []struct {
		file, src, word string
	}{
		{"a.sysml", "package P { metadata def M; #M /* class */ class C; }", "class"},
		{"a.sysml", "package P { abstract /* class */ class C; }", "class"},
		{"a.sysml", "package P { metadata def M; #M // class\nclass C; }", "class"},
		{"a.sysml", "package P { metadata def M; metadata def N; #M /* class */ #N class C; }", "class"},
		{"a.kerml", "package P { abstract /* part */ part def D; }", "part"},
	} {
		_, _, all := notationDiagnostics(t, tc.file, tc.src)
		var got []source.Span
		for _, d := range all {
			if d.Code == CodeKerMLNotation || d.Code == CodeSysMLNotation {
				got = append(got, d.Span)
			}
		}
		want := source.Span{Offset: strings.LastIndex(tc.src, tc.word), Len: len(tc.word)}
		if len(got) != 1 || got[0] != want {
			t.Errorf("%q: spans = %+v, want [%+v]", tc.src, got, want)
		}
	}
}

// A compound keyword (`assoc struct`) is spanned whole, however its words are
// separated.
func TestDeclarationKeywordSpanCoversCompoundKeyword(t *testing.T) {
	for _, src := range []string{
		"package P { part def X; part def Y; assoc struct A { end a : X; end b : Y; } }",
		"package P { part def X; part def Y; assoc   struct A { end a : X; end b : Y; } }",
	} {
		_, _, got := notationDiagnostics(t, "a.sysml", src)
		if len(got) != 1 {
			t.Fatalf("%q: got %d diagnostics %+v, want 1", src, len(got), got)
		}
		at := strings.Index(src, "assoc")
		want := source.Span{Offset: at, Len: strings.Index(src, "struct") + len("struct") - at}
		if got[0].Span != want {
			t.Errorf("%q: span = %+v, want %+v", src, got[0].Span, want)
		}
	}
}
