package lexer

import "github.com/Open-MBEE/OpenSysML/internal/core/source"

// Lexer is a hand-written, pull-based scanner. Call Next() repeatedly.
type Lexer struct {
	sf  *source.SourceFile // retained for Plan 02 (parser diagnostics/positions)
	src []byte
	pos int // current byte offset
}

// New creates a Lexer over a source file.
func New(sf *source.SourceFile) *Lexer {
	return &Lexer{sf: sf, src: sf.Bytes()}
}

// Next returns the next token, including trivia tokens. At end of input it
// returns EOF repeatedly (idempotent).
func (lx *Lexer) Next() Token {
	if lx.pos >= len(lx.src) {
		return Token{Kind: EOF, Span: source.Span{Offset: len(lx.src), Len: 0}}
	}
	start := lx.pos
	c := lx.src[lx.pos]

	switch {
	case c == ' ' || c == '\t' || c == '\r' || c == '\n':
		return lx.scanWhitespace(start)
	case c == '/' && lx.peek(1) == '/' && lx.peek(2) == '*':
		return lx.scanMLNote(start) // //* ... */
	case c == '/' && lx.peek(1) == '/':
		return lx.scanSLNote(start) // // ...
	case c == '/' && lx.peek(1) == '*':
		return lx.scanBlockComment(start) // /* ... */
	case isIdentStart(c):
		return lx.scanIdentOrKeyword(start)
	case c == '\'':
		return lx.scanQuoted(start, '\'', UnrestrictedName)
	case c >= '0' && c <= '9':
		return lx.scanNumber(start)
	case c == '.' && lx.peek(1) >= '0' && lx.peek(1) <= '9':
		return lx.scanNumber(start) // leading-dot real: .5
	case c == '"':
		return lx.scanQuoted(start, '"', String)
	}

	// Operators & punctuation (longest match first).
	switch c {
	case ':':
		if lx.peek(1) == ':' {
			if lx.peek(2) == '>' {
				lx.pos += 3
				return Token{Kind: ColonColonGt, Span: lx.span(start)}
			}
			lx.pos += 2
			return Token{Kind: ColonColon, Span: lx.span(start)}
		}
		if lx.peek(1) == '=' {
			lx.pos += 2
			return Token{Kind: ColonEq, Span: lx.span(start)}
		}
		if lx.peek(1) == '>' {
			if lx.peek(2) == '>' {
				lx.pos += 3
				return Token{Kind: ColonGtGt, Span: lx.span(start)}
			}
			lx.pos += 2
			return Token{Kind: ColonGt, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Colon, Span: lx.span(start)}
	case '-':
		if lx.peek(1) == '>' {
			lx.pos += 2
			return Token{Kind: Arrow, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Minus, Span: lx.span(start)}
	case '.':
		if lx.peek(1) == '?' {
			lx.pos += 2
			return Token{Kind: DotQuestion, Span: lx.span(start)}
		}
		if lx.peek(1) == '.' {
			lx.pos += 2
			return Token{Kind: DotDot, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Dot, Span: lx.span(start)}
	case '*':
		if lx.peek(1) == '*' {
			lx.pos += 2
			return Token{Kind: StarStar, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Star, Span: lx.span(start)}
	case '=':
		if lx.peek(1) == '=' && lx.peek(2) == '=' {
			lx.pos += 3
			return Token{Kind: EqEqEq, Span: lx.span(start)}
		}
		if lx.peek(1) == '=' {
			lx.pos += 2
			return Token{Kind: EqEq, Span: lx.span(start)}
		}
		if lx.peek(1) == '>' {
			lx.pos += 2
			return Token{Kind: EqGt, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Eq, Span: lx.span(start)}
	case '!':
		if lx.peek(1) == '=' && lx.peek(2) == '=' {
			lx.pos += 3
			return Token{Kind: NotEqEq, Span: lx.span(start)}
		}
		if lx.peek(1) == '=' {
			lx.pos += 2
			return Token{Kind: NotEq, Span: lx.span(start)}
		}
		// bare '!' is not a defined token
		lx.pos++
		return Token{Kind: Error, Span: lx.span(start)}
	case '<':
		if lx.peek(1) == '=' {
			lx.pos += 2
			return Token{Kind: Le, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Lt, Span: lx.span(start)}
	case '>':
		if lx.peek(1) == '=' {
			lx.pos += 2
			return Token{Kind: Ge, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Gt, Span: lx.span(start)}
	case '?':
		if lx.peek(1) == '?' {
			lx.pos += 2
			return Token{Kind: QuestionQ, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Question, Span: lx.span(start)}
	case '@':
		if lx.peek(1) == '@' {
			lx.pos += 2
			return Token{Kind: AtAt, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: At, Span: lx.span(start)}
	case '/':
		// // and /* already handled as trivia in Task 6; this is division
		lx.pos++
		return Token{Kind: Slash, Span: lx.span(start)}
	}

	// Single-char punctuation with a fixed 1:1 mapping.
	if k, ok := singleCharKind[c]; ok {
		lx.pos++
		return Token{Kind: k, Span: lx.span(start)}
	}

	// Unrecognized byte(s): coalesce a maximal run of bytes that cannot start
	// any known token into a single Error token. Guarantees progress.
	return lx.scanError(start)
}

// scanError consumes a maximal run of bytes that cannot begin a valid token,
// emitting a single Error token. Always advances at least one byte.
func (lx *Lexer) scanError(start int) Token {
	lx.pos++ // consume the offending byte (guarantees progress)
	for lx.pos < len(lx.src) && !canStartToken(lx.src[lx.pos]) {
		lx.pos++
	}
	return Token{Kind: Error, Span: lx.span(start)}
}

// canStartToken reports whether c can begin some valid token. Used only to
// bound Error coalescing; conservative (false positives just end the run early).
func canStartToken(c byte) bool {
	switch {
	case c == ' ' || c == '\t' || c == '\r' || c == '\n':
		return true
	case isIdentStart(c):
		return true
	case isDigit(c):
		return true
	case c == '\'' || c == '"':
		return true
	}
	switch c {
	case ':', '-', '.', '*', '=', '!', '<', '>', '?', '@', '/',
		'|', '&', '+', '%', '^', '~', '#', '(', ')', '[', ']',
		'{', '}', ',', '$', ';':
		return true
	}
	return false
}

var singleCharKind = map[byte]Kind{
	'|': Pipe, '&': Amp, '+': Plus, '%': Percent, '^': Caret, '~': Tilde,
	'#': Hash, '(': LParen, ')': RParen, '[': LBracket, ']': RBracket,
	'{': LBrace, '}': RBrace, ',': Comma, '$': Dollar, ';': Semicolon,
}

func (lx *Lexer) scanWhitespace(start int) Token {
	for lx.pos < len(lx.src) {
		c := lx.src[lx.pos]
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			break
		}
		lx.pos++
	}
	return Token{Kind: Whitespace, Span: lx.span(start)}
}

func (lx *Lexer) scanSLNote(start int) Token {
	lx.pos += 2 // consume "//"
	for lx.pos < len(lx.src) && lx.src[lx.pos] != '\n' && lx.src[lx.pos] != '\r' {
		lx.pos++
	}
	// SL_NOTE includes the trailing line terminator: KerMLExpressions.xtext
	// SL_NOTE: '//' (...)? ('\r'? '\n')?  -- so \r?\n is part of the token span.
	if lx.pos < len(lx.src) && lx.src[lx.pos] == '\r' {
		lx.pos++
	}
	if lx.pos < len(lx.src) && lx.src[lx.pos] == '\n' {
		lx.pos++
	}
	return Token{Kind: SLNote, Span: lx.span(start)}
}

func (lx *Lexer) scanMLNote(start int) Token {
	lx.pos += 3 // consume "//*"
	closed := lx.consumeUntilStarSlash()
	return Token{Kind: MLNote, Span: lx.span(start), Unterminated: !closed}
}

func (lx *Lexer) scanBlockComment(start int) Token {
	lx.pos += 2 // consume "/*"
	closed := lx.consumeUntilStarSlash()
	return Token{Kind: RegularComment, Span: lx.span(start), Unterminated: !closed}
}

// scanQuoted scans a quoted literal delimited by quote (' or "), flagging escapes
// outside the terminal's set; unterminated (newline/EOF) emits Error over what was read.
func (lx *Lexer) scanQuoted(start int, quote byte, kind Kind) Token {
	lx.pos++ // opening quote
	badEscape := false
	for lx.pos < len(lx.src) {
		c := lx.src[lx.pos]
		switch {
		case c == '\\':
			// escape: consume backslash + next char if present
			lx.pos++
			if lx.pos < len(lx.src) {
				if !IsEscapeChar(lx.src[lx.pos]) {
					badEscape = true
				}
				lx.pos++
			}
		case c == quote:
			lx.pos++ // closing quote
			return Token{Kind: kind, Span: lx.span(start), BadEscape: badEscape}
		case c == '\n' || c == '\r':
			// unterminated on this line
			return Token{Kind: Error, Span: lx.span(start)}
		default:
			lx.pos++
		}
	}
	// reached EOF without closing quote
	return Token{Kind: Error, Span: lx.span(start)}
}

// consumeUntilStarSlash advances until it consumes a closing "*/", reporting
// whether it found one; without one it stops at EOF.
func (lx *Lexer) consumeUntilStarSlash() bool {
	for lx.pos < len(lx.src) {
		if lx.src[lx.pos] == '*' && lx.peek(1) == '/' {
			lx.pos += 2
			return true
		}
		lx.pos++
	}
	return false
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func (lx *Lexer) scanIdentOrKeyword(start int) Token {
	lx.pos++ // first char already known to be identStart
	for lx.pos < len(lx.src) && isIdentCont(lx.src[lx.pos]) {
		lx.pos++
	}
	sp := lx.span(start)
	// The map index over a []byte->string conversion is optimized by the
	// compiler to avoid allocating; the keyword path shares the canonical
	// string stored in the map, so no token allocates its own copy.
	if kw, ok := keywords[string(lx.src[start:lx.pos])]; ok {
		return Token{Kind: Keyword, Span: sp, KeywordID: kw}
	}
	return Token{Kind: Identifier, Span: sp}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// scanNumber scans Decimal or Real starting at start.
func (lx *Lexer) scanNumber(start int) Token {
	isReal := false

	// leading-dot real (.5) — start is '.'
	if lx.src[lx.pos] == '.' {
		isReal = true
		lx.pos++ // '.'
		lx.consumeDigits()
		lx.consumeExponent(&isReal)
		return Token{Kind: Real, Span: lx.span(start)}
	}

	lx.consumeDigits() // integer part

	// fractional part: '.' only if followed by a digit (avoids 1..2 and 1.)
	if lx.pos < len(lx.src) && lx.src[lx.pos] == '.' &&
		lx.pos+1 < len(lx.src) && isDigit(lx.src[lx.pos+1]) {
		isReal = true
		lx.pos++ // '.'
		lx.consumeDigits()
	}

	lx.consumeExponent(&isReal)

	if isReal {
		return Token{Kind: Real, Span: lx.span(start)}
	}
	return Token{Kind: Decimal, Span: lx.span(start)}
}

func (lx *Lexer) consumeDigits() {
	for lx.pos < len(lx.src) && isDigit(lx.src[lx.pos]) {
		lx.pos++
	}
}

// consumeExponent consumes an (e|E)(+|-)?[0-9]+ suffix if present, setting real.
func (lx *Lexer) consumeExponent(isReal *bool) {
	if lx.pos >= len(lx.src) {
		return
	}
	c := lx.src[lx.pos]
	if c != 'e' && c != 'E' {
		return
	}
	// lookahead: need optional sign then at least one digit, else not an exponent
	i := lx.pos + 1
	if i < len(lx.src) && (lx.src[i] == '+' || lx.src[i] == '-') {
		i++
	}
	if i >= len(lx.src) || !isDigit(lx.src[i]) {
		return // not a valid exponent; leave 'e' for identifier/other handling
	}
	*isReal = true
	lx.pos = i
	lx.consumeDigits()
}

// peek returns the byte at pos+n without advancing, or 0 if out of range.
func (lx *Lexer) peek(n int) byte {
	i := lx.pos + n
	if i >= len(lx.src) {
		return 0
	}
	return lx.src[i]
}

// span builds a Span from a start offset to the current pos.
func (lx *Lexer) span(start int) source.Span {
	return source.Span{Offset: start, Len: lx.pos - start}
}
