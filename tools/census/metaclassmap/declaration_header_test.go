package metaclassmap

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestDeclarationHeaderAnonymousWordRun(t *testing.T) {
	for _, tc := range []struct {
		name, text, want string
	}{
		{"return", "return : T = x;", "return "},
		{"perform", "then perform body;", "then perform body"},
		{"assert", "assert constraint { true }", "assert constraint "},
		{"send", "send new Show(...) to screen", "send new Show"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sym := &symbols.Symbol{DeclSpan: source.Span{Offset: 0, Len: len(tc.text)}}
			start, end, ok := declarationHeader(sym, tc.text)
			if !ok || start != 0 || tc.text[start:end] != tc.want {
				t.Fatalf("header = %q (%d:%d, %t), want %q", tc.text[start:end], start, end, ok, tc.want)
			}
		})
	}
}

func TestDeclarationHeaderIsCappedAtDeclarationEnd(t *testing.T) {
	const text = "send new Show(...) to screen"
	sym := &symbols.Symbol{DeclSpan: source.Span{Offset: 0, Len: len("send new")}}
	start, end, ok := declarationHeader(sym, text)
	if !ok || start != 0 || text[start:end] != "send new" {
		t.Fatalf("header = %q (%d:%d, %t), want %q", text[start:end], start, end, ok, "send new")
	}
}

func TestAnonymousDeclarationHeadersUseActualDeclarationStarts(t *testing.T) {
	const doc = "headers.sysml"
	const text = `package Demo {
	calc def C {
		in fn : SampledFunction;
		return : Anything[0..*] = fn.samples.domainValue;
	}
	action def A {
		private action initialization
			assign index := 1;
		then private action whileLoop
			while index <= 1 {
				assign index := 2;
				then perform body;
				then assign index := 3;
			}
		assert constraint { true }
		send new Show() to screen;
	}
}`
	p := parser.New(source.New(doc, []byte(text)))
	parsed := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocument(doc, parsed)
	root := idx.DocumentRoot(doc)
	seen := map[*symbols.Symbol]bool{}
	found := map[string]bool{}
	wants := map[string]string{
		"return": "return ",
		"assert": "assert constraint ",
		"send":   "send new Show",
	}
	var walk func(*symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil {
			return
		}
		for _, sym := range scope.AllMembers() {
			if !seen[sym] {
				seen[sym] = true
				if sym.NameSpan.Len == 0 {
					for keyword, wantHeader := range wants {
						offset := strings.Index(text, keyword)
						if sym.DeclSpan.Offset != offset {
							continue
						}
						start, end, ok := declarationHeader(sym, text)
						if !ok || text[start:end] != wantHeader {
							t.Errorf("%T at %d header = %q, want %q", sym.Decl, sym.DeclSpan.Offset,
								text[start:end], wantHeader)
						}
						found[keyword] = true
					}
				}
				walk(sym.Scope)
			}
		}
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	walk(root)
	for keyword := range wants {
		if !found[keyword] {
			t.Errorf("no anonymous declaration starts at %q", keyword)
		}
	}
}
