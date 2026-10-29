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

// classifyBraces decides the kind of every `{` in toks, in order: exprBody
// where an operand goes, else multiBody once a `;` or nested body is inside.
func classifyBraces(toks []lexer.Token) []braceKind {
	var kinds []braceKind
	type openBrace struct {
		at            int // index into kinds
		code, comment bool
	}
	var stack []*openBrace
	brackets := 0
	var last lexer.Kind
	invoked := false // inside the (qualified) name an `->` invokes
	top := func() *openBrace { return stack[len(stack)-1] }
	for _, tok := range toks {
		if tok.IsTrivia() {
			continue
		}
		if len(stack) > 0 {
			switch tok.Kind {
			case lexer.RegularComment:
				top().comment = true
			case lexer.RBrace:
			default:
				top().code = true
			}
		}
		switch tok.Kind {
		case lexer.LParen, lexer.LBracket:
			brackets++
		case lexer.RParen, lexer.RBracket:
			if brackets > 0 {
				brackets--
			}
		case lexer.LBrace:
			kind := singleBody
			switch {
			case brackets > 0, len(stack) > 0 && kinds[top().at] == exprBody,
				incompleteBefore[last], invoked, last == 0, last == lexer.Semicolon, last == lexer.LBrace:
				kind = exprBody
			case len(stack) > 0:
				kinds[top().at] = multiBody
			}
			kinds = append(kinds, kind)
			stack = append(stack, &openBrace{at: len(kinds) - 1})
		case lexer.RBrace:
			if len(stack) > 0 {
				if b := top(); b.comment && b.code && kinds[b.at] == singleBody {
					kinds[b.at] = multiBody
				}
				stack = stack[:len(stack)-1]
			}
		case lexer.Semicolon:
			if len(stack) > 0 && kinds[top().at] == singleBody {
				kinds[top().at] = multiBody
			}
		}
		switch tok.Kind {
		case lexer.Arrow:
			invoked = true
		case lexer.Identifier, lexer.ColonColon:
		default:
			invoked = false
		}
		if tok.Kind != lexer.RegularComment {
			last = tok.Kind
		}
	}
	return kinds
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
		return !joinsStatement(tok, f.lastCode)
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
		kind := exprBody
		if len(f.braces) > 0 {
			kind, f.braces = f.braces[0], f.braces[1:]
		}
		f.open = append(f.open, kind)
		if kind == multiBody {
			return breakNext
		}
	case lexer.RBrace:
		if n := len(f.open); n > 0 {
			kind := f.open[n-1]
			f.open = f.open[:n-1]
			if kind != exprBody {
				return breakUnlessJoined
			}
		}
	case lexer.Semicolon:
		if f.innermost() != exprBody {
			return breakUnlessJoined
		}
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

// joinsStatement reports whether tok stays on the line of the `}` or `;`
// (after) before it: `} then Idle;`, `} else {`, `{in x; x} + 1`, but `; -x`.
func joinsStatement(tok lexer.Token, after lexer.Kind) bool {
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
		case "then", "else", "until":
			return true
		case "and", "or", "xor", "implies", "istype", "hastype", "as":
			return after == lexer.RBrace
		}
	}
	return false
}
