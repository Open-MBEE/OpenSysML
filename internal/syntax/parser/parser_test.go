package parser

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func newParser(src string) *Parser {
	sf := source.New("test", []byte(src))
	return New(sf)
}

func TestPeekSkipsTrivia(t *testing.T) {
	p := newParser("  \n // note\n part")
	tok := p.peek()
	if tok.Kind != lexer.Keyword || tok.KeywordID != "part" {
		t.Fatalf("peek = %+v", tok)
	}
}

func TestAdvanceAdvances(t *testing.T) {
	p := newParser("a b")
	first := p.advance()
	second := p.peek()
	if first.Kind != lexer.Identifier {
		t.Fatalf("first = %+v", first)
	}
	if second.Kind != lexer.Identifier || p.src.Text(second.Span) != "b" {
		t.Fatalf("second = %+v", second)
	}
}

func TestPeekN(t *testing.T) {
	p := newParser("a :: b")
	if p.peek().Kind != lexer.Identifier {
		t.Fatal("peek0")
	}
	if p.peekN(1).Kind != lexer.ColonColon {
		t.Fatalf("peek1 = %+v", p.peekN(1))
	}
	if p.peekN(2).Kind != lexer.Identifier {
		t.Fatalf("peek2 = %+v", p.peekN(2))
	}
}

func TestExpectRecordsDiagnostic(t *testing.T) {
	p := newParser("a")
	p.advance() // consume 'a'
	tok, ok := p.expect(lexer.Semicolon, "expected ';'")
	if ok {
		t.Fatal("expected failure at EOF")
	}
	if len(p.Diagnostics) != 1 || p.Diagnostics[0].Message != "expected ';'" {
		t.Fatalf("diags = %+v", p.Diagnostics)
	}
	if tok.Kind != lexer.EOF {
		t.Fatalf("tok = %+v", tok)
	}
}

func TestTriviaRecordsNotesAndCommentsOnly(t *testing.T) {
	p := newParser("  // line note\n //* block note */ /* comment */ part")
	if tok := p.peek(); tok.Kind != lexer.Keyword || tok.KeywordID != "part" {
		t.Fatalf("peek = %+v", tok)
	}
	kinds := []ast.TriviaKind{}
	for _, tr := range p.triv {
		kinds = append(kinds, tr.Kind)
	}
	want := []ast.TriviaKind{ast.TriviaLineNote, ast.TriviaBlockNote, ast.TriviaComment}

	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("trivia kinds = %v, want %v", kinds, want)
	}
}

// A restored checkpoint replays the trivia the abandoned attempt consumed —
// each exactly once, in order — instead of losing what takeTrivia attached to
// a discarded node.
func TestRestoreKeepsTriviaOnce(t *testing.T) {
	p := newParser("/* a */ x /* b */ y")
	p.peek() // lexes '/* a */'
	cp := p.checkpoint()
	defer p.release()
	p.takeTrivia() // the discarded node's leading trivia
	if p.peekN(1).Kind != lexer.Identifier {
		t.Fatalf("peekN(1) = %+v, want 'y'", p.peekN(1))
	}
	p.restore(cp)
	if len(p.triv) != 2 {
		t.Fatalf("trivia = %v, want 2 entries", p.triv)
	}
	if got := p.src.Text(p.triv[0].Span); got != "/* a */" {
		t.Errorf("trivia[0] = %q, want \"/* a */\"", got)
	}
	if got := p.src.Text(p.triv[1].Span); got != "/* b */" {
		t.Errorf("trivia[1] = %q, want \"/* b */\"", got)
	}
}

func TestAtEOF(t *testing.T) {
	p := newParser("")
	if !p.atEOF() {
		t.Fatal("empty should be EOF")
	}
}
