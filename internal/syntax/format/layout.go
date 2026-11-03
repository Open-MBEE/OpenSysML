package format

import "github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"

// braceKind is what a `{` opens, for statement layout.
type braceKind uint8

const (
	// exprBody is the body of an expression — `->forAll { in x; ... }`, the
	// `{ ... }` an operator takes, anything inside brackets — laid out as written.
	exprBody braceKind = iota
	// singleBody is a statement body holding no statement: empty, or one result
	// expression, as `assert constraint { x > 0 }`. It stays on its line.
	singleBody
	// multiBody is a statement body with statements in it, one per line.
	multiBody
)

// breakKind is the line break a token owes the token after it.
type breakKind uint8

const (
	noBreak           breakKind = iota
	breakNext                   // a new line before whatever comes next
	breakUnlessJoined           // ...unless the next token continues the statement
)

// openBrace is a `{` not closed yet: where its kind is in the kinds so far,
// and whether code or a comment has appeared inside it.
type openBrace struct {
	at            int // index into kinds
	code, comment bool
}

// braceClassifier is the state classifyBraces carries from token to token.
type braceClassifier struct {
	kinds    []braceKind
	stack    []*openBrace
	brackets int
	last     lexer.Kind
	invoked  bool // inside the (qualified) name an `->` invokes
}

func (c *braceClassifier) top() *openBrace { return c.stack[len(c.stack)-1] }

// noteContent records on the innermost open brace what kind of token is inside it.
func (c *braceClassifier) noteContent(kind lexer.Kind) {
	if len(c.stack) == 0 {
		return
	}
	switch kind {
	case lexer.RegularComment:
		c.top().comment = true
	case lexer.RBrace:
	default:
		c.top().code = true
	}
}

// open classifies a `{`: exprBody where an operand goes, else singleBody, and
// a body opened inside a statement body makes that one multiBody.
func (c *braceClassifier) open() {
	kind := singleBody
	switch {
	case c.brackets > 0, len(c.stack) > 0 && c.kinds[c.top().at] == exprBody,
		incompleteBefore[c.last], c.invoked, c.last == 0, c.last == lexer.Semicolon, c.last == lexer.LBrace:
		kind = exprBody
	case len(c.stack) > 0:
		c.kinds[c.top().at] = multiBody
	}
	c.kinds = append(c.kinds, kind)
	c.stack = append(c.stack, &openBrace{at: len(c.kinds) - 1})
}

// close pops a `}`: a single body holding both code and a comment is laid out
// as statements, so the comment keeps its own line.
func (c *braceClassifier) close() {
	if len(c.stack) == 0 {
		return
	}
	if b := c.top(); b.comment && b.code && c.kinds[b.at] == singleBody {
		c.kinds[b.at] = multiBody
	}
	c.stack = c.stack[:len(c.stack)-1]
}

// classifyBraces decides the kind of every `{` in toks, in order: exprBody
// where an operand goes, else multiBody once a `;` or nested body is inside.
func classifyBraces(toks []lexer.Token) []braceKind {
	var c braceClassifier
	for _, tok := range toks {
		if tok.IsTrivia() {
			continue
		}
		c.noteContent(tok.Kind)
		switch tok.Kind {
		case lexer.LParen, lexer.LBracket:
			c.brackets++
		case lexer.RParen, lexer.RBracket:
			if c.brackets > 0 {
				c.brackets--
			}
		case lexer.LBrace:
			c.open()
		case lexer.RBrace:
			c.close()
		case lexer.Semicolon:
			if len(c.stack) > 0 && c.kinds[c.top().at] == singleBody {
				c.kinds[c.top().at] = multiBody
			}
		}
		switch tok.Kind {
		case lexer.Arrow:
			c.invoked = true
		case lexer.Identifier, lexer.ColonColon:
		default:
			c.invoked = false
		}
		if tok.Kind != lexer.RegularComment {
			c.last = tok.Kind
		}
	}
	return c.kinds
}

// breakBefore reports whether statement layout puts tok on a new line; a note
// the author wrote at the end of the line stays there.
func (f *formatter) breakBefore(tok lexer.Token) bool {
	if tok.IsTrivia() {
		return false
	}
	switch f.owed {
	case breakNext:
		return true
	case breakUnlessJoined:
		if tok.Kind == lexer.LBrace {
			// `default {true} {` goes on; a result expression after `;` does not.
			return f.lastCode != lexer.RBrace
		}
		return !joinsStatement(tok, f.lastCode, f.afterEntry)
	}
	return tok.Kind == lexer.RBrace && f.innermost() == multiBody
}

// trackStatement records what tok opens or closes and returns the line break
// it owes the next token under statement layout.
func (f *formatter) trackStatement(tok lexer.Token) breakKind {
	switch tok.Kind {
	case lexer.SLNote, lexer.MLNote:
		return f.owed
	case lexer.RegularComment:
		// Not trivia: the body of a doc, comment or rep element, or a comment
		// element on its own, each of which ends there.
		if f.innermost() == multiBody {
			return breakNext
		}
		return f.owed
	case lexer.LBrace:
		f.stmtLen = 0
		kind := exprBody
		if len(f.braces) > 0 {
			kind, f.braces = f.braces[0], f.braces[1:]
		}
		f.open = append(f.open, kind)
		if kind == multiBody {
			return breakNext
		}
	case lexer.RBrace:
		f.stmtLen = 0
		if n := len(f.open); n > 0 {
			kind := f.open[n-1]
			f.open = f.open[:n-1]
			if kind != exprBody {
				return breakUnlessJoined
			}
		}
	case lexer.Semicolon:
		f.afterEntry = f.stmtLen == 1 && f.stmtKeyword == "entry"
		f.stmtLen = 0
		if f.innermost() != exprBody {
			return breakUnlessJoined
		}
	default:
		if f.stmtLen == 0 {
			f.stmtKeyword = tok.KeywordID
		}
		f.stmtLen++
	}
	return noBreak
}

// innermost is the kind of the innermost open brace; the top level holds
// statements, so it counts as a multiBody.
func (f *formatter) innermost() braceKind {
	if n := len(f.open); n > 0 {
		return f.open[n-1]
	}
	return multiBody
}

// joinsStatement reports whether tok stays on the line of the `}` or `;` (after)
// before it: `} then Idle;`, `entry; then Off;`, `{in x; x} + 1`, but `; -x`.
func joinsStatement(tok lexer.Token, after lexer.Kind, afterEntry bool) bool {
	switch tok.Kind {
	case lexer.Semicolon, lexer.Comma, lexer.RParen, lexer.RBracket:
		return true
	case lexer.Pipe, lexer.Amp, lexer.EqEq, lexer.NotEq, lexer.EqEqEq, lexer.NotEqEq,
		lexer.Lt, lexer.Gt, lexer.Le, lexer.Ge, lexer.DotDot,
		lexer.Plus, lexer.Minus, lexer.Star, lexer.Slash, lexer.Percent,
		lexer.StarStar, lexer.Caret, lexer.Dot, lexer.Arrow, lexer.DotQuestion,
		lexer.Question, lexer.QuestionQ:
		return after == lexer.RBrace
	case lexer.Keyword:
		switch tok.KeywordID {
		case "then":
			// A succession chain `action a; then action b;` is one per line.
			return after == lexer.RBrace || afterEntry
		case "else", "until":
			return true
		case "and", "or", "xor", "implies", "istype", "hastype", "as":
			return after == lexer.RBrace
		}
	}
	return false
}
