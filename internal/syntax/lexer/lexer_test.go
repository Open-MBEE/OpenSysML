package lexer

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// lex is a test helper: collects all tokens including trivia until EOF.
func lex(t *testing.T, input string) []Token {
	t.Helper()
	sf := source.New("t.sysml", []byte(input))
	lx := New(sf)
	var toks []Token
	for {
		tok := lx.Next()
		toks = append(toks, tok)
		if tok.Kind == EOF {
			return toks
		}
		if len(toks) > 10000 {
			t.Fatal("lexer did not terminate")
		}
	}
}

func TestEmptyInputYieldsEOF(t *testing.T) {
	toks := lex(t, "")
	if len(toks) != 1 || toks[0].Kind != EOF {
		t.Fatalf("empty input toks = %v, want single EOF", toks)
	}
	if toks[0].Span.Offset != 0 {
		t.Fatalf("EOF offset = %d, want 0", toks[0].Span.Offset)
	}
}

func TestEOFIsIdempotent(t *testing.T) {
	sf := source.New("t.sysml", []byte(""))
	lx := New(sf)
	_ = lx.Next()
	if k := lx.Next().Kind; k != EOF {
		t.Fatalf("second Next() after EOF = %v, want EOF", k)
	}
}

// kinds extracts just the Kind sequence for compact assertions.
func kinds(toks []Token) []Kind {
	ks := make([]Kind, len(toks))
	for i, t := range toks {
		ks[i] = t.Kind
	}
	return ks
}

func eq(a, b []Kind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWhitespace(t *testing.T) {
	toks := lex(t, "   \t\n ")
	want := []Kind{Whitespace, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
	if toks[0].Span.Len != 6 {
		t.Fatalf("ws span len = %d, want 6", toks[0].Span.Len)
	}
}

func TestSLNote(t *testing.T) {
	toks := lex(t, "// hello\n")
	if !eq(kinds(toks), []Kind{SLNote, EOF}) {
		t.Fatalf("kinds = %v", kinds(toks))
	}
}

func TestMLNoteBeatsSLNote(t *testing.T) {
	toks := lex(t, "//* note */")
	if !eq(kinds(toks), []Kind{MLNote, EOF}) {
		t.Fatalf("kinds = %v, want MLNote EOF", kinds(toks))
	}
}

func TestRegularComment(t *testing.T) {
	toks := lex(t, "/* c */")
	if !eq(kinds(toks), []Kind{RegularComment, EOF}) {
		t.Fatalf("kinds = %v, want RegularComment EOF", kinds(toks))
	}
}

func TestUnterminatedBlockComment(t *testing.T) {
	toks := lex(t, "/* open")
	if toks[0].Kind != RegularComment {
		t.Fatalf("first kind = %v, want RegularComment", toks[0].Kind)
	}
	if toks[0].Span.Len != 7 {
		t.Fatalf("span len = %d, want 7 (to EOF)", toks[0].Span.Len)
	}
}

func TestSLNoteSpanIncludesTerminator(t *testing.T) {
	// "// hi\n" is 6 bytes; SL_NOTE per grammar includes the trailing \n.
	toks := lex(t, "// hi\n")
	if toks[0].Kind != SLNote {
		t.Fatalf("kind = %v, want SLNote", toks[0].Kind)
	}
	if toks[0].Span.Len != 6 {
		t.Fatalf("SLNote span len = %d, want 6", toks[0].Span.Len)
	}
}

func TestSLNoteNoTerminatorAtEOF(t *testing.T) {
	// No trailing newline: span is just the "// hi" (5 bytes).
	toks := lex(t, "// hi")
	if toks[0].Kind != SLNote {
		t.Fatalf("kind = %v, want SLNote", toks[0].Kind)
	}
	if toks[0].Span.Len != 5 {
		t.Fatalf("SLNote span len = %d, want 5", toks[0].Span.Len)
	}
}

func TestRegularCommentSpanLen(t *testing.T) {
	toks := lex(t, "/* c */")
	if toks[0].Span.Len != 7 {
		t.Fatalf("RegularComment span len = %d, want 7", toks[0].Span.Len)
	}
}

func TestMLNoteSpanLen(t *testing.T) {
	toks := lex(t, "//* note */")
	if toks[0].Kind != MLNote {
		t.Fatalf("kind = %v, want MLNote", toks[0].Kind)
	}
	if toks[0].Span.Len != 11 {
		t.Fatalf("MLNote span len = %d, want 11", toks[0].Span.Len)
	}
}

func TestUnterminatedMLNote(t *testing.T) {
	toks := lex(t, "//* open")
	if toks[0].Kind != MLNote {
		t.Fatalf("kind = %v, want MLNote", toks[0].Kind)
	}
	if toks[0].Span.Len != 8 {
		t.Fatalf("MLNote span len = %d, want 8 (to EOF)", toks[0].Span.Len)
	}
}

func TestIdentifier(t *testing.T) {
	toks := lex(t, "Engine _x9 abc")
	want := []Kind{Identifier, Whitespace, Identifier, Whitespace, Identifier, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestKeyword(t *testing.T) {
	toks := lex(t, "part def package")
	for i, ki := range []int{0, 2, 4} {
		if toks[ki].Kind != Keyword {
			t.Fatalf("token %d kind = %v, want Keyword", i, toks[ki].Kind)
		}
	}
	if toks[0].KeywordID != "part" {
		t.Fatalf("KeywordID = %q, want part", toks[0].KeywordID)
	}
}

func TestKeywordPrefixIsIdentifier(t *testing.T) {
	toks := lex(t, "partial")
	if toks[0].Kind != Identifier {
		t.Fatalf("kind = %v, want Identifier", toks[0].Kind)
	}
}

func TestCRLFWhitespaceIsOneToken(t *testing.T) {
	toks := lex(t, "\r\n\r\n")
	if !eq(kinds(toks), []Kind{Whitespace, EOF}) {
		t.Fatalf("kinds = %v, want Whitespace EOF", kinds(toks))
	}
	if toks[0].Span.Len != 4 {
		t.Fatalf("ws span len = %d, want 4", toks[0].Span.Len)
	}
}

func TestUnrestrictedName(t *testing.T) {
	toks := lex(t, "'my name'")
	if !eq(kinds(toks), []Kind{UnrestrictedName, EOF}) {
		t.Fatalf("kinds = %v", kinds(toks))
	}
	if toks[0].Span.Len != 9 {
		t.Fatalf("span len = %d, want 9", toks[0].Span.Len)
	}
}

func TestUnrestrictedNameWithEscape(t *testing.T) {
	toks := lex(t, `'a\'b'`)
	if !eq(kinds(toks), []Kind{UnrestrictedName, EOF}) {
		t.Fatalf("kinds = %v, want UnrestrictedName EOF", kinds(toks))
	}
	if toks[0].Span.Len != 6 {
		t.Fatalf("span len = %d, want 6", toks[0].Span.Len)
	}
}

func TestUnterminatedUnrestrictedName(t *testing.T) {
	toks := lex(t, "'open\n")
	if toks[0].Kind != Error {
		t.Fatalf("kind = %v, want Error", toks[0].Kind)
	}
}

func TestDecimal(t *testing.T) {
	toks := lex(t, "42")
	if !eq(kinds(toks), []Kind{Decimal, EOF}) {
		t.Fatalf("kinds = %v", kinds(toks))
	}
}

func TestReal(t *testing.T) {
	for _, in := range []string{"1.5", ".5", "1e3", "1.5e-2", "2E+10", "1.0e5"} {
		toks := lex(t, in)
		if !eq(kinds(toks), []Kind{Real, EOF}) {
			t.Fatalf("input %q kinds = %v, want Real EOF", in, kinds(toks))
		}
	}
}

func TestRangeNotReal(t *testing.T) {
	// 1..2 must be Decimal DotDot Decimal, not a malformed real.
	toks := lex(t, "1..2")
	want := []Kind{Decimal, DotDot, Decimal, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestTrailingDot(t *testing.T) {
	// "1." → Decimal then Dot (no digit after dot)
	toks := lex(t, "1.")
	want := []Kind{Decimal, Dot, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestString(t *testing.T) {
	toks := lex(t, `"hello world"`)
	if !eq(kinds(toks), []Kind{String, EOF}) {
		t.Fatalf("kinds = %v", kinds(toks))
	}
}

func TestStringWithEscape(t *testing.T) {
	toks := lex(t, `"a\"b\n"`)
	if !eq(kinds(toks), []Kind{String, EOF}) {
		t.Fatalf("kinds = %v, want String EOF", kinds(toks))
	}
}

func TestUnterminatedString(t *testing.T) {
	toks := lex(t, `"open`)
	if toks[0].Kind != Error {
		t.Fatalf("kind = %v, want Error", toks[0].Kind)
	}
}

func TestOperators(t *testing.T) {
	cases := []struct {
		in   string
		want []Kind
	}{
		{"::", []Kind{ColonColon, EOF}},
		{":", []Kind{Colon, EOF}},
		{"->", []Kind{Arrow, EOF}},
		{".?", []Kind{DotQuestion, EOF}},
		{"..", []Kind{DotDot, EOF}},
		{".", []Kind{Dot, EOF}},
		{"**", []Kind{StarStar, EOF}},
		{"*", []Kind{Star, EOF}},
		{"==", []Kind{EqEq, EOF}},
		{"===", []Kind{EqEqEq, EOF}},
		{"!=", []Kind{NotEq, EOF}},
		{"!==", []Kind{NotEqEq, EOF}},
		{"<=", []Kind{Le, EOF}},
		{">=", []Kind{Ge, EOF}},
		{"??", []Kind{QuestionQ, EOF}},
		{"?", []Kind{Question, EOF}},
		{"@@", []Kind{AtAt, EOF}},
		{"@", []Kind{At, EOF}},
		{"|&+-%^~#()[]{},$=;<>", []Kind{
			Pipe, Amp, Plus, Minus, Percent, Caret, Tilde, Hash,
			LParen, RParen, LBracket, RBracket, LBrace, RBrace, Comma,
			Dollar, Eq, Semicolon, Lt, Gt, EOF,
		}},
	}
	for _, c := range cases {
		toks := lex(t, c.in)
		if !eq(kinds(toks), c.want) {
			t.Errorf("input %q kinds = %v, want %v", c.in, kinds(toks), c.want)
		}
	}
}

func TestOperatorGreedyBoundaries(t *testing.T) {
	cases := []struct {
		in   string
		want []Kind
	}{
		{"====", []Kind{EqEqEq, Eq, EOF}},
		{"!===", []Kind{NotEqEq, Eq, EOF}},
		{"...", []Kind{DotDot, Dot, EOF}},
		{".?.", []Kind{DotQuestion, Dot, EOF}},
		{"->->", []Kind{Arrow, Arrow, EOF}},
	}
	for _, c := range cases {
		toks := lex(t, c.in)
		if !eq(kinds(toks), c.want) {
			t.Errorf("input %q kinds = %v, want %v", c.in, kinds(toks), c.want)
		}
	}
}

func TestOperatorSpanLengths(t *testing.T) {
	cases := []struct {
		in      string
		wantLen int
	}{
		{"===", 3},
		{"!==", 3},
		{"<=", 2},
		{"::", 2},
		{"..", 2},
		{".?", 2},
		{"->", 2},
		{":", 1},
		{".", 1},
	}
	for _, c := range cases {
		toks := lex(t, c.in)
		if len(toks) < 1 {
			t.Fatalf("input %q produced no tokens", c.in)
		}
		if toks[0].Span.Len != c.wantLen {
			t.Errorf("input %q first token Span.Len = %d, want %d", c.in, toks[0].Span.Len, c.wantLen)
		}
	}
}

func TestOperatorMixedStreams(t *testing.T) {
	cases := []struct {
		in   string
		want []Kind
	}{
		{"a->b", []Kind{Identifier, Arrow, Identifier, EOF}},
		{"1<=2", []Kind{Decimal, Le, Decimal, EOF}},
		{"!x", []Kind{Error, Identifier, EOF}},
	}
	for _, c := range cases {
		toks := lex(t, c.in)
		if !eq(kinds(toks), c.want) {
			t.Errorf("input %q kinds = %v, want %v", c.in, kinds(toks), c.want)
		}
	}
}

func TestErrorCoalescing(t *testing.T) {
	// A run of unrecognized bytes becomes ONE Error token.
	toks := lex(t, "\x00\x01\x02")
	if !eq(kinds(toks), []Kind{Error, EOF}) {
		t.Fatalf("kinds = %v, want Error EOF", kinds(toks))
	}
	if toks[0].Span.Len != 3 {
		t.Fatalf("error span len = %d, want 3", toks[0].Span.Len)
	}
}

func TestErrorThenValid(t *testing.T) {
	toks := lex(t, "\x00part")
	want := []Kind{Error, Keyword, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestAlwaysMakesProgress(t *testing.T) {
	// Every non-EOF token must have Len >= 1 so parsers can't loop forever.
	toks := lex(t, "part\x00'unterminated")
	for _, tk := range toks {
		if tk.Kind != EOF && tk.Span.Len < 1 {
			t.Fatalf("token %v has zero width", tk)
		}
	}
}

// TestCanStartTokenCoversPunctuation guards the hand-maintained sync between
// the operator dispatch, singleCharKind, and canStartToken. Every byte that
// begins a real token must be reported startable, else scanError would wrongly
// swallow it into an Error run.
func TestCanStartTokenCoversPunctuation(t *testing.T) {
	// First bytes of all multi/single-char operators and punctuation.
	starts := []byte(":-.*=!<>?@/|&+%^~#()[]{},$;")
	for _, b := range starts {
		if !canStartToken(b) {
			t.Errorf("canStartToken(%q) = false, want true", string(b))
		}
	}
	for b := range singleCharKind {
		if !canStartToken(b) {
			t.Errorf("singleCharKind key %q not covered by canStartToken", string(b))
		}
	}
}

func TestCompoundRelationshipOperators(t *testing.T) {
	cases := []struct {
		src  string
		want []Kind
	}{
		{":>", []Kind{ColonGt, EOF}},
		{":>>", []Kind{ColonGtGt, EOF}},
		{"::>", []Kind{ColonColonGt, EOF}},
		{"=>", []Kind{EqGt, EOF}},
		{":", []Kind{Colon, EOF}},
		{"::", []Kind{ColonColon, EOF}},
		{"=", []Kind{Eq, EOF}},
		{">", []Kind{Gt, EOF}},
		{"<x>", []Kind{Lt, Identifier, Gt, EOF}},
		{":> >", []Kind{ColonGt, Gt, EOF}},
		{"= 1", []Kind{Eq, Decimal, EOF}},
	}
	for _, tc := range cases {
		lx := New(source.New("<t>", []byte(tc.src)))
		var got []Kind
		for {
			tok := lx.Next()
			if tok.IsTrivia() {
				continue
			}
			got = append(got, tok.Kind)
			if tok.Kind == EOF {
				break
			}
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%q: got %v want %v", tc.src, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("%q: token %d got %v want %v", tc.src, i, got[i], tc.want[i])
			}
		}
	}
}

// TestBehavioralKeywords verifies lexer recognizes all SysML v2 behavioral
// keywords for action control-flow and state machine parsing (Task 6).
func TestBehavioralKeywords(t *testing.T) {
	src := source.New("test", []byte("first fork join merge decide then state entry exit transition after when accept parallel"))
	l := New(src)

	expectedKeywords := []string{
		"first", "fork", "join", "merge", "decide", "then", // Action
		"state", "entry", "exit", "transition", // State
		"after", "when", "accept", "parallel", // Trigger and state body
	}
	for i, expected := range expectedKeywords {
		tok := l.Next()
		// skip whitespace between keywords
		for tok.IsTrivia() {
			tok = l.Next()
		}
		if tok.Kind != Keyword {
			t.Errorf("token %d: expected keyword, got %v", i, tok.Kind)
		}
		text := src.Text(tok.Span)
		if text != expected {
			t.Errorf("token %d: expected %q, got %q", i, expected, text)
		}
	}
}

// TestContextualWordsAreIdentifiers pins the words the grammar uses in one
// position only: the lexer hands them over as names and the parser matches them
// contextually, so a model may declare a feature with any of them.
func TestContextualWordsAreIdentifiers(t *testing.T) {
	for _, word := range []string{
		"on", "var", "point", "chain",
		// Words that are a literal in none of the pinned grammars: our own state
		// and action notation (docs/reference/grammar/conformance-audit.md).
		"choice", "deep", "defer", "done", "final", "history",
		"initial", "junction", "region", "shallow",
	} {
		src := source.New("test", []byte(word))
		tok := New(src).Next()
		if tok.Kind != Identifier {
			t.Errorf("%q: expected Identifier, got %v", word, tok.Kind)
		}
		if got := src.Text(tok.Span); got != word {
			t.Errorf("%q: got text %q", word, got)
		}
	}
}

func TestTokenArrow(t *testing.T) {
	src := source.New("test", []byte("a -> b"))
	l := New(src)

	l.Next() // 'a'
	l.Next() // whitespace
	tok := l.Next()

	if tok.Kind != Arrow {
		t.Errorf("expected Arrow, got %v", tok.Kind)
	}
	if src.Text(tok.Span) != "->" {
		t.Errorf("expected '->', got %q", src.Text(tok.Span))
	}
}

func TestMinusNotArrow(t *testing.T) {
	src := source.New("test", []byte("a - b"))
	l := New(src)

	l.Next() // 'a'
	l.Next() // whitespace
	tok := l.Next()

	if tok.Kind != Minus {
		t.Errorf("expected Minus, got %v", tok.Kind)
	}
}
