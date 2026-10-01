package migrate

import (
	"strconv"
	"strings"
	"unicode"
)

// The OCL subset a «TableExpressionColumn» expression is lowered from:
// navigation chains, collection operations with an iterator body, oclAsType
// casts, = and <> comparisons, and/or/not, and literals. Each expression is
// parsed to an immutable tree, then lowered to a cell expression over the row
// by the metaproperties the v2 model carries.

type oclKind int

const (
	oclVariable oclKind = iota // name
	oclString                  // text
	oclInteger                 // text
	oclBoolean                 // text
	oclNavigate                // src.name, or an implicit self.name with src nil
	oclCall                    // src.name(args) or src->name(args) when arrow
	oclIterate                 // src->name(variable | args[0])
	oclBinary                  // args[0] name args[1]
	oclNot                     // not args[0]
)

// oclNode is one node of a parsed expression; text is its source spelling.
type oclNode struct {
	kind     oclKind
	name     string
	literal  string
	src      *oclNode
	args     []*oclNode
	variable string
	arrow    bool
	text     string
}

// oclToken is a lexical token: a punctuation or keyword spelled by kind, an
// identifier, a string or an integer literal.
type oclToken struct {
	kind string
	text string
	pos  int
}

// oclLex splits an expression into tokens; why says what it could not read.
func oclLex(src string) (tokens []oclToken, why string) {
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '\'':
			j := i + 1
			var sb strings.Builder
			for j < len(src) && src[j] != '\'' {
				if src[j] == '\\' && j+1 < len(src) {
					j++
				}
				sb.WriteByte(src[j])
				j++
			}
			if j >= len(src) {
				return nil, "the string starting at " + strconv.Quote(src[i:]) + " is not closed"
			}
			tokens = append(tokens, oclToken{"string", sb.String(), i})
			i = j + 1
		case c >= '0' && c <= '9':
			j := i
			for j < len(src) && src[j] >= '0' && src[j] <= '9' {
				j++
			}
			tokens = append(tokens, oclToken{"integer", src[i:j], i})
			i = j
		case c == '_' || c == '$' || unicode.IsLetter(rune(c)):
			j := i
			for j < len(src) && (src[j] == '_' || src[j] == '$' || unicode.IsLetter(rune(src[j])) || unicode.IsDigit(rune(src[j]))) {
				j++
			}
			word := src[i:j]
			kind := "ident"
			switch word {
			case "and", "or", "xor", "implies", "not", "true", "false", "self", "null":
				kind = word
			}
			tokens = append(tokens, oclToken{kind, word, i})
			i = j
		case strings.HasPrefix(src[i:], "->"), strings.HasPrefix(src[i:], "<>"),
			strings.HasPrefix(src[i:], "<="), strings.HasPrefix(src[i:], ">="):
			tokens = append(tokens, oclToken{src[i : i+2], src[i : i+2], i})
			i += 2
		case strings.ContainsRune(".()|,=<>", rune(c)):
			tokens = append(tokens, oclToken{string(c), string(c), i})
			i++
		default:
			return nil, "the character " + strconv.Quote(string(c)) + " has no place in an OCL expression"
		}
	}
	return tokens, ""
}

// oclParser is a recursive-descent parser over the tokens of one expression.
type oclParser struct {
	src    string
	tokens []oclToken
	at     int
}

// parseOCL parses an OCL expression; why quotes what it could not parse.
func parseOCL(src string) (*oclNode, string) {
	tokens, why := oclLex(src)
	if why != "" {
		return nil, why
	}
	p := &oclParser{src: src, tokens: tokens}
	n, why := p.expression()
	if why != "" {
		return nil, why
	}
	if p.at < len(p.tokens) {
		return nil, "the text " + strconv.Quote(p.rest()) + " follows the expression"
	}
	if n == nil {
		return nil, "the expression is empty"
	}
	return n, ""
}

func (p *oclParser) peek() oclToken {
	if p.at < len(p.tokens) {
		return p.tokens[p.at]
	}
	return oclToken{kind: "end", pos: len(p.src)}
}

func (p *oclParser) next() oclToken {
	t := p.peek()
	if p.at < len(p.tokens) {
		p.at++
	}
	return t
}

func (p *oclParser) rest() string {
	return strings.TrimSpace(p.src[p.peek().pos:])
}

// spanned fills in the source text of n from start to the current position.
func (p *oclParser) spanned(n *oclNode, start int) *oclNode {
	end := len(p.src)
	if p.at < len(p.tokens) {
		end = p.tokens[p.at].pos
	}
	n.text = strings.TrimSpace(p.src[start:end])
	return n
}

func (p *oclParser) expect(kind string) string {
	t := p.peek()
	if t.kind != kind {
		if t.kind == "end" {
			return "the expression ends where " + strconv.Quote(kind) + " was expected"
		}
		return strconv.Quote(kind) + " was expected at " + strconv.Quote(p.rest())
	}
	p.next()
	return ""
}

func (p *oclParser) expression() (*oclNode, string) {
	return p.binary(0)
}

// oclLevels orders the binary operators from loosest to tightest.
var oclLevels = [][]string{{"implies"}, {"or", "xor"}, {"and"}, {"=", "<>", "<", ">", "<=", ">="}}

func (p *oclParser) binary(level int) (*oclNode, string) {
	if level == len(oclLevels) {
		return p.unary()
	}
	start := p.peek().pos
	left, why := p.binary(level + 1)
	if why != "" {
		return nil, why
	}
	for {
		t := p.peek()
		if !contains(oclLevels[level], t.kind) {
			return left, ""
		}
		p.next()
		right, why := p.binary(level + 1)
		if why != "" {
			return nil, why
		}
		left = p.spanned(&oclNode{kind: oclBinary, name: t.kind, args: []*oclNode{left, right}}, start)
	}
}

func (p *oclParser) unary() (*oclNode, string) {
	start := p.peek().pos
	if p.peek().kind == "not" {
		p.next()
		operand, why := p.unary()
		if why != "" {
			return nil, why
		}
		return p.spanned(&oclNode{kind: oclNot, name: "not", args: []*oclNode{operand}}, start), ""
	}
	return p.postfix()
}

func (p *oclParser) postfix() (*oclNode, string) {
	start := p.peek().pos
	n, why := p.primary()
	if why != "" {
		return nil, why
	}
	for {
		switch p.peek().kind {
		case ".", "->":
			arrow := p.next().kind == "->"
			name := p.next()
			if name.kind != "ident" {
				return nil, "a name was expected after " + strconv.Quote(p.src[start:name.pos])
			}
			if p.peek().kind != "(" {
				if arrow {
					return nil, "the collection operation " + strconv.Quote(name.text) + " is not applied"
				}
				n = p.spanned(&oclNode{kind: oclNavigate, name: name.text, src: n}, start)
				continue
			}
			p.next()
			call, why := p.call(n, name.text, arrow, start)
			if why != "" {
				return nil, why
			}
			n = call
		default:
			return n, ""
		}
	}
}

// call parses the arguments of src.name( or src->name(, an iterator body when
// a variable and a bar open them.
func (p *oclParser) call(src *oclNode, name string, arrow bool, start int) (*oclNode, string) {
	if p.peek().kind == "ident" && p.at+1 < len(p.tokens) && p.tokens[p.at+1].kind == "|" {
		variable := p.next().text
		p.next()
		body, why := p.expression()
		if why != "" {
			return nil, why
		}
		if why := p.expect(")"); why != "" {
			return nil, why
		}
		if !arrow {
			return nil, "the iterator " + strconv.Quote(name) + " is applied with a dot, not an arrow"
		}
		return p.spanned(&oclNode{kind: oclIterate, name: name, src: src, variable: variable, args: []*oclNode{body}}, start), ""
	}
	var args []*oclNode
	for p.peek().kind != ")" {
		if len(args) > 0 {
			if why := p.expect(","); why != "" {
				return nil, why
			}
		}
		arg, why := p.expression()
		if why != "" {
			return nil, why
		}
		args = append(args, arg)
	}
	p.next()
	return p.spanned(&oclNode{kind: oclCall, name: name, src: src, arrow: arrow, args: args}, start), ""
}

func (p *oclParser) primary() (*oclNode, string) {
	start := p.peek().pos
	t := p.next()
	switch t.kind {
	case "string":
		return p.spanned(&oclNode{kind: oclString, literal: t.text}, start), ""
	case "integer":
		return p.spanned(&oclNode{kind: oclInteger, literal: t.text}, start), ""
	case "true", "false":
		return p.spanned(&oclNode{kind: oclBoolean, literal: t.text}, start), ""
	case "self":
		return p.spanned(&oclNode{kind: oclVariable, name: "self"}, start), ""
	case "ident":
		if p.peek().kind == "(" {
			p.next()
			return p.call(nil, t.text, false, start)
		}
		return p.spanned(&oclNode{kind: oclVariable, name: t.text}, start), ""
	case "(":
		n, why := p.expression()
		if why != "" {
			return nil, why
		}
		if why := p.expect(")"); why != "" {
			return nil, why
		}
		return n, ""
	case "end":
		return nil, "the expression ends where a value was expected"
	}
	return nil, strconv.Quote(t.text) + " cannot start a value"
}

// conjuncts flattens a conjunction into its terms.
func (n *oclNode) conjuncts() []*oclNode {
	if n.kind == oclBinary && n.name == "and" {
		return append(n.args[0].conjuncts(), n.args[1].conjuncts()...)
	}
	return []*oclNode{n}
}

// uncast drops oclAsType casts and self-navigation wrappers from n.
func (n *oclNode) uncast() *oclNode {
	for n.kind == oclCall && n.name == "oclAsType" && n.src != nil {
		n = n.src
	}
	return n
}

// isVariable tells whether n reads the variable name, through casts.
func (n *oclNode) isVariable(name string) bool {
	n = n.uncast()
	return n.kind == oclVariable && n.name == name
}

// path reads n as a navigation chain v.a.b from the variable v, through casts;
// ok is false when n is anything else.
func (n *oclNode) path() (variable string, members []string, ok bool) {
	n = n.uncast()
	switch n.kind {
	case oclVariable:
		return n.name, nil, true
	case oclNavigate:
		if n.src == nil {
			return "self", []string{n.name}, true
		}
		variable, members, ok = n.src.path()
		return variable, append(members, n.name), ok
	}
	return "", nil, false
}
