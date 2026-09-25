// Package format re-indents SysML v2 / KerML source text.
//
// The formatter works on the token stream rather than on the AST. Printing an
// AST back to text would drop everything the AST does not model — comments,
// notes, and any construct the parser records as an ErrorNode — so a formatter
// built that way cannot safely be pointed at a user's file. Working on tokens
// keeps every lexeme and only rewrites the whitespace between them.
//
// The rules are deliberately conservative: line breaks the author wrote are
// preserved, so the formatter never joins or splits a line. It fixes
// indentation, trailing whitespace, runs of blank lines, and the padding just
// inside brackets and before separators. A line that continues the statement
// above it — the author broke a declaration or expression across lines — is
// indented one level deeper than that statement, so it still reads as a
// continuation rather than as a new statement. Line endings are emitted in the
// document's own dominant style, so a CRLF file stays CRLF throughout.
package format

import (
	"bytes"
	"errors"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Options controls the emitted indentation.
type Options struct {
	// IndentWidth is the number of columns per nesting level (default 4).
	IndentWidth int
	// UseTabs indents with one tab per level instead of spaces.
	UseTabs bool
}

// DefaultOptions matches the indentation used by the models in examples/.
var DefaultOptions = Options{IndentWidth: 4}

// ErrNotIdempotent reports that formatting would have changed the token stream.
// It signals a bug in the formatter, never bad input: callers should present the
// source unchanged rather than a mangled file.
var ErrNotIdempotent = errors.New("format: rewrite would change the token stream")

// Source formats src, which must be the contents of name. It returns the
// formatted bytes, or ErrNotIdempotent (with src unchanged) if the result would
// not lex back to the same tokens.
func Source(name string, src []byte, opts Options) ([]byte, error) {
	if opts.IndentWidth <= 0 {
		opts.IndentWidth = DefaultOptions.IndentWidth
	}
	out := (&formatter{src: src, opts: opts, nl: dominantNewline(src)}).run(tokenize(name, src))
	if !sameTokens(name, src, out) {
		return src, ErrNotIdempotent
	}
	return out, nil
}

// dominantNewline reports the line ending to emit: CRLF only when the document
// already uses it more than bare LF.
func dominantNewline(src []byte) string {
	crlf := bytes.Count(src, []byte("\r\n"))
	if crlf > bytes.Count(src, []byte("\n"))-crlf {
		return "\r\n"
	}
	return "\n"
}

func tokenize(name string, src []byte) []lexer.Token {
	lx := lexer.New(source.New(name, src))
	var toks []lexer.Token
	for {
		tok := lx.Next()
		if tok.Kind == lexer.EOF {
			return toks
		}
		toks = append(toks, tok)
	}
}

type formatter struct {
	src  []byte
	opts Options

	buf   bytes.Buffer
	depth int
	nl    string // line ending to emit

	atLineStart bool // nothing written on the current line yet
	pendingNL   int  // newlines seen since the last emitted token
	pendingWS   bool // same-line whitespace seen since the last emitted token
	prev        lexer.Token
	started     bool
	lastCode    lexer.Kind // last non-comment token: what a line break interrupts
	openComment bool       // the last token is a comment the lexer never saw closed
}

func (f *formatter) run(toks []lexer.Token) []byte {
	f.atLineStart = true
	for _, tok := range toks {
		if tok.Kind == lexer.Whitespace {
			text := f.text(tok)
			if n := strings.Count(text, "\n"); n > 0 {
				f.pendingNL += n
			} else {
				f.pendingWS = true
			}
			continue
		}
		f.emit(tok)
	}
	out := f.buf.Bytes()
	if f.started && !f.openComment {
		// Exactly one trailing newline, whether or not the last token was a line
		// comment that carried its own. An unterminated comment runs to the end of
		// the file and would swallow it, changing that token's own text.
		out = append(bytes.TrimRight(out, "\r\n"), f.nl...)
	}
	return out
}

// emit writes one non-whitespace token, together with the whitespace that
// should precede it.
func (f *formatter) emit(tok lexer.Token) {
	if tok.Kind == lexer.RBrace {
		f.depth--
		if f.depth < 0 {
			f.depth = 0
		}
	}

	switch {
	case f.pendingNL > 0:
		if f.started {
			// At most one blank line between tokens, counting the newline a line
			// comment token already wrote as part of its own text.
			want := 1
			if f.pendingNL > 1 {
				want = 2
			}
			for have := f.trailingNewlines(); have < want; have++ {
				f.buf.WriteString(f.nl)
			}
		}
		f.atLineStart = true
	case f.pendingWS && f.started && !f.atLineStart && f.wantSpace(tok):
		f.buf.WriteByte(' ')
	}
	f.pendingNL, f.pendingWS = 0, false

	if f.atLineStart {
		f.writeIndent(f.continues(tok))
	}
	text := normalizeNewlines(f.text(tok), f.nl)
	f.buf.WriteString(text)

	if tok.Kind == lexer.LBrace {
		f.depth++
	}
	f.prev, f.started, f.atLineStart = tok, true, false
	f.openComment = tok.Unterminated
	if !isComment(tok.Kind) {
		// A trailing comment interrupts nothing, so the token before it is
		// still what the next line continues.
		f.lastCode = tok.Kind
	}
	if i := strings.LastIndexByte(text, '\n'); i >= 0 {
		// A multi-line note or block comment is emitted verbatim; the tokens
		// after it start from wherever it ended.
		f.atLineStart = strings.TrimSpace(text[i+1:]) == ""
	}
}

// trailingNewlines counts the line endings already at the end of the buffer; a
// line comment token includes its terminating newline in its own text.
func (f *formatter) trailingNewlines() int {
	b := f.buf.Bytes()
	n := 0
	for bytes.HasSuffix(b, []byte(f.nl)) {
		b = b[:len(b)-len(f.nl)]
		n++
	}
	return n
}

// normalizeNewlines rewrites the line endings inside a token's own text — a
// multi-line note, or a line comment carrying its terminator — to nl.
func normalizeNewlines(text, nl string) string {
	if !strings.Contains(text, "\n") {
		return text
	}
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", nl)
}

// wantSpace reports whether whitespace the author wrote before tok is kept.
// Only pairs that cannot lex differently once joined are tightened, so removing
// the space can never merge two tokens into one.
func (f *formatter) wantSpace(tok lexer.Token) bool {
	switch tok.Kind {
	case lexer.Semicolon, lexer.Comma, lexer.RParen, lexer.RBracket:
		return false
	}
	switch f.prev.Kind {
	case lexer.LParen, lexer.LBracket:
		return false
	}
	return true
}

// continues reports whether the line tok begins carries on the line above it,
// which is the case when the break falls inside a phrase: either the token
// before it cannot end one, or tok itself cannot start one. A break the author
// made anywhere else — after a declaration's name, say — is left at the
// statement's own level, so only breaks the formatter can be sure about move.
func (f *formatter) continues(tok lexer.Token) bool {
	switch tok.Kind {
	case lexer.LBrace, lexer.RBrace, lexer.RParen, lexer.RBracket:
		// A delimiter on its own line belongs to the line that opened it rather
		// than one level in from it.
		return false
	}
	return incompleteBefore[f.lastCode] || continuesAfter[tok.Kind]
}

func isComment(k lexer.Kind) bool {
	switch k {
	case lexer.SLNote, lexer.MLNote, lexer.RegularComment:
		return true
	}
	return false
}

// infix are the tokens that join two operands or two clauses, so a line break
// on either side of one falls inside a phrase.
var infix = []lexer.Kind{
	lexer.Pipe, lexer.Amp, lexer.EqEq, lexer.NotEq, lexer.EqEqEq, lexer.NotEqEq,
	lexer.Lt, lexer.Gt, lexer.Le, lexer.Ge, lexer.DotDot,
	lexer.Plus, lexer.Minus, lexer.Star, lexer.Slash, lexer.Percent,
	lexer.StarStar, lexer.Caret, lexer.Dot, lexer.Arrow, lexer.DotQuestion,
	lexer.Comma, lexer.ColonColon, lexer.Eq, lexer.Colon, lexer.ColonEq,
	lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt, lexer.EqGt,
}

// incompleteBefore are the tokens that cannot end a phrase: an infix operator,
// an unclosed bracket, or a prefix awaiting its operand.
var incompleteBefore = kindSet(infix, []lexer.Kind{
	lexer.LParen, lexer.LBracket, lexer.Question, lexer.QuestionQ,
	lexer.Tilde, lexer.At, lexer.AtAt, lexer.Hash, lexer.Dollar,
})

// continuesAfter are the tokens that cannot start a statement, so a line
// beginning with one continues the line above.
var continuesAfter = kindSet(infix)

func kindSet(groups ...[]lexer.Kind) map[lexer.Kind]bool {
	set := map[lexer.Kind]bool{}
	for _, group := range groups {
		for _, kind := range group {
			set[kind] = true
		}
	}
	return set
}

func (f *formatter) writeIndent(continuation bool) {
	depth := f.depth
	if continuation {
		depth++
	}
	if f.opts.UseTabs {
		f.buf.WriteString(strings.Repeat("\t", depth))
		return
	}
	f.buf.WriteString(strings.Repeat(" ", depth*f.opts.IndentWidth))
}

func (f *formatter) text(tok lexer.Token) string {
	return string(f.src[tok.Span.Offset:tok.Span.End()])
}

// sameTokens reports whether before and after lex to the same sequence of
// non-whitespace tokens with the same text: the formatter's safety net.
func sameTokens(name string, before, after []byte) bool {
	a, b := significant(name, before), significant(name, after)
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

func significant(name string, src []byte) []string {
	toks := tokenize(name, src)
	out := make([]string, 0, len(toks))
	for _, tok := range toks {
		if tok.Kind == lexer.Whitespace {
			continue
		}
		// Compared with line endings normalized, since emitting the document's
		// dominant ending is an intended rewrite.
		text := normalizeNewlines(string(src[tok.Span.Offset:tok.Span.End()]), "\n")
		if tok.Kind == lexer.SLNote {
			// A line comment's text includes its terminating newline, which
			// the formatter supplies when the file did not end with one.
			text = strings.TrimSuffix(text, "\n")
		}
		out = append(out, tok.Kind.String()+" "+text)
	}
	return out
}
