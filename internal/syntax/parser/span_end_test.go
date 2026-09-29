package parser

import (
	"strings"
	"testing"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// spanEndModel spaces its tokens apart, with a line comment and blank lines in
// between, so a span that runs on to the next token's start shows as trailing
// whitespace.
const spanEndModel = `package P {
    part x : Strat ;
    part def D :> Missing  {
        attribute a : Real   = 1 +  2   ;   // trailing note

    }
    comment Named about D /* the body is the comment's own */
    state s {
        entry ;  then a ;
        state a ;
        transition t first a accept Strat then a ;
    }
    action act {
        action step1 ;
        then step2 ;
        action step2 ;
    }
}
`

// Every node's span ends at the end of its last token (or of the comment body
// it owns), never over the whitespace or line comment that follows.
func TestNodeSpansEndAtTheirLastToken(t *testing.T) {
	src := []byte(spanEndModel)
	p := New(source.New("span.sysml", src))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", p.Diagnostics)
	}
	checked := 0
	ast.Inspect(file, func(n ast.Node) bool {
		sp := n.Span()
		if sp.Len == 0 || sp.End() > len(src) {
			return true
		}
		checked++
		text := string(src[sp.Offset:sp.End()])
		if last := rune(text[len(text)-1]); unicode.IsSpace(last) {
			t.Errorf("%T span %q ends in whitespace", n, text)
		}
		if strings.Contains(text, "// trailing note") && !strings.HasSuffix(strings.TrimSpace(text), "}") {
			t.Errorf("%T span %q covers the line comment after it", n, text)
		}
		return true
	})
	if checked == 0 {
		t.Fatal("no node spans were checked")
	}
}

// A reference's span is its name alone, whatever follows it: a typing's type
// before `;`, a specialization's target before `{`, and an accepted signal
// before `then`.
func TestReferenceSpansStopAtTheName(t *testing.T) {
	src := []byte(spanEndModel)
	p := New(source.New("span.sysml", src))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", p.Diagnostics)
	}
	want := map[string]int{"Strat": 0, "Missing": 0}
	ast.Inspect(file, func(n ast.Node) bool {
		qn, ok := n.(*ast.QualifiedName)
		if !ok {
			return true
		}
		name := qn.Text()
		if _, tracked := want[name]; !tracked {
			return true
		}
		sp := qn.Span()
		if got := string(src[sp.Offset:sp.End()]); got != name {
			t.Errorf("reference %s spans %q, want exactly its name", name, got)
		}
		want[name]++
		return true
	})
	if want["Strat"] != 2 || want["Missing"] != 1 {
		t.Errorf("references seen = %v, want Strat twice (typing, accept) and Missing once", want)
	}
}
