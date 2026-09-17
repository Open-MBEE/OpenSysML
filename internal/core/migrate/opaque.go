package migrate

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file translates a bounded subset of the opaque languages a v1 model
// carries — JavaScript-family scripts and English guards — into v2 expressions
// and assignments. Anything outside the subset is a typed refusal that names
// the offending token; a body is translated whole or not at all.

// refusalKind classifies why an opaque body has no v2 form.
type refusalKind int

const (
	refusedLanguage  refusalKind = iota // a language the subset does not cover
	refusedSyntax                       // text that is not the subset's syntax
	refusedConstruct                    // a construct the subset leaves out: a loop, `new`, indexing, ...
	refusedCall                         // a function call outside the table
	refusedName                         // a name nothing readable answers
	refusedType                         // operands or an assignment whose types disagree
	refusedContext                      // `this` where no context object is known
)

// refusal is why an opaque body is not translated: its kind, the token at
// fault and the reason, spelled for the migration report.
type refusal struct {
	kind  refusalKind
	token string
	why   string
}

// note spells the refusal for a report entry or a comment.
func (r *refusal) note() string {
	var text string
	switch r.kind {
	case refusedLanguage:
		text = "the language " + strconv.Quote(r.token) + " is not translated"
	case refusedSyntax:
		text = "the text " + strconv.Quote(r.token) + " is not expression syntax"
	case refusedConstruct:
		text = "the construct " + strconv.Quote(r.token) + " is outside the translated subset"
	case refusedCall:
		text = "the call " + strconv.Quote(r.token) + " is not in the translated function table"
	case refusedName:
		text = "the name " + strconv.Quote(r.token) + " resolves to nothing readable"
	case refusedType:
		text = "the types at " + strconv.Quote(r.token) + " disagree"
	case refusedContext:
		text = strconv.Quote(r.token) + " names no known object here"
	}
	if r.why != "" {
		text += ": " + r.why
	}
	return text
}

// opaqueRef is what a scope answers for a name: the v2 expression reading it
// and the scalar it holds ("" when unknown or not a scalar), plural for a collection.
type opaqueRef struct {
	expr   string
	scalar string
	plural bool
}

// opaqueScope answers what the names of an opaque body mean where it is read;
// a path starting with `this` asks for a feature of the context object.
type opaqueScope interface {
	feature(path []string, write bool) (opaqueRef, *refusal)
}

// translated is a v2 expression the translator produced, with what it knows
// of its type; atomic is true when it needs no parentheses as an operand, and
// lit names the literal kind when the expression is one literal.
type translated struct {
	expr   string
	scalar string
	plural bool
	atomic bool
	lit    string
}

// operand writes t as the operand of an operator.
func (t translated) operand() string {
	if t.atomic {
		return t.expr
	}
	return "(" + t.expr + ")"
}

// dialect is the family an opaque language belongs to.
type dialect int

const (
	dialectNone    dialect = iota // a language the translator does not read
	dialectScript                 // JavaScript, ECMAScript, Java or no language
	dialectEnglish                // English or natural-language text
)

// dialectOf classifies an opaque body's language.
func dialectOf(lang string) dialect {
	l := strings.ToLower(strings.TrimSpace(lang))
	switch {
	case l == "", strings.HasPrefix(l, "javascript"), strings.HasPrefix(l, "ecmascript"),
		l == "js", strings.HasPrefix(l, "java"), strings.HasPrefix(l, "rhino"), strings.HasPrefix(l, "nashorn"):
		return dialectScript
	case l == "english", l == "natural", strings.HasPrefix(l, "natural language"), l == "text", l == "plain":
		return dialectEnglish
	}
	return dialectNone
}

// translateExpr translates body as one expression read in sc; want names the
// scalar it must yield, "" for any. The expression is complete or refused.
func translateExpr(body, lang string, sc opaqueScope, want string) (translated, *refusal) {
	d := dialectOf(lang)
	if d == dialectNone {
		return translated{}, &refusal{kind: refusedLanguage, token: lang}
	}
	if _, err := wholeExprIn(body, d, anyScope{}); err != nil && err.kind != refusedType {
		return translated{}, err
	}
	t, err := wholeExprIn(body, d, sc)
	if err != nil {
		return translated{}, err
	}
	if want != "" && t.scalar != "" && !assignableScalar(want, t) {
		return translated{}, &refusal{kind: refusedType, token: body,
			why: "the expression is a " + t.scalar + ", not the " + want + " wanted"}
	}
	return t, nil
}

// translateStatements translates body as a sequence of script statements into
// the lines of a v2 action body: local declarations and assignments.
func translateStatements(body, lang string, sc opaqueScope) ([]string, *refusal) {
	d := dialectOf(lang)
	switch d {
	case dialectNone:
		return nil, &refusal{kind: refusedLanguage, token: lang}
	case dialectEnglish:
		return nil, &refusal{kind: refusedLanguage, token: lang, why: "prose has no statements to write"}
	}
	if _, err := statementsIn(body, d, anyScope{}); err != nil && err.kind != refusedType {
		return nil, err
	}
	return statementsIn(body, d, sc)
}

// wholeExprIn parses body as one expression of dialect d, its names answered by sc.
func wholeExprIn(body string, d dialect, sc opaqueScope) (translated, *refusal) {
	p, err := newOpaqueParser(body, d, sc)
	if err != nil {
		return translated{}, err
	}
	t, err := p.wholeExpr()
	if err != nil && d == dialectEnglish {
		if named, ok := p.wholeName(body); ok {
			return named, nil
		}
	}
	return t, err
}

// statementsIn parses body as statements of dialect d, its names answered by sc.
func statementsIn(body string, d dialect, sc opaqueScope) ([]string, *refusal) {
	p, err := newOpaqueParser(body, d, sc)
	if err != nil {
		return nil, err
	}
	return p.statements()
}

// anyScope answers every name with an unknown type, so a body's shape is judged
// first: a construct or syntax outside the subset is refused ahead of a name.
type anyScope struct{}

func (anyScope) feature(path []string, _ bool) (opaqueRef, *refusal) {
	return opaqueRef{expr: strings.Join(path, ".")}, nil
}

// tokKind is the kind of a token of an opaque body.
type tokKind int

const (
	tokEOF tokKind = iota
	tokNewline
	tokNumber
	tokString
	tokIdent
	tokPunct
)

// token is one token of an opaque body; text is its source spelling, or for a
// string its unescaped content.
type token struct {
	kind tokKind
	text string
}

// puncts lists the multi-character operators, longest first.
var puncts = []string{
	"===", "!==", "**=", "&&", "||", "==", "!=", "<=", ">=", "+=", "-=", "*=", "/=", "%=", "++", "--", "**",
	"+", "-", "*", "/", "%", "<", ">", "=", "!", "?", ":", "(", ")", ".", ",", ";", "[", "]", "{", "}",
}

// lexOpaque scans body into tokens; comments are dropped and newlines kept, as
// a script ends a statement at one.
func lexOpaque(body string) ([]token, *refusal) {
	var toks []token
	s := body
	for s != "" {
		r, size := utf8.DecodeRuneInString(s)
		switch {
		case r == '\n':
			toks = append(toks, token{tokNewline, "\n"})
			s = s[size:]
			continue
		case unicode.IsSpace(r):
			s = s[size:]
			continue
		case strings.HasPrefix(s, "//"):
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i:]
			} else {
				s = ""
			}
			continue
		case strings.HasPrefix(s, "/*"):
			i := strings.Index(s[2:], "*/")
			if i < 0 {
				return nil, &refusal{kind: refusedSyntax, token: "/*", why: "the comment is not closed"}
			}
			s = s[i+4:]
			continue
		case r == '"' || r == '\'':
			text, rest, ok := lexString(s, r)
			if !ok {
				return nil, &refusal{kind: refusedSyntax, token: string(r), why: "the string is not closed"}
			}
			toks = append(toks, token{tokString, text})
			s = rest
			continue
		case unicode.IsDigit(r) || (r == '.' && len(s) > 1 && isDigit(s[1])):
			text, rest := lexNumber(s)
			toks = append(toks, token{tokNumber, text})
			s = rest
			continue
		case unicode.IsLetter(r) || r == '_' || r == '$':
			i := size
			for i < len(s) {
				c, n := utf8.DecodeRuneInString(s[i:])
				if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' && c != '$' {
					break
				}
				i += n
			}
			toks = append(toks, token{tokIdent, s[:i]})
			s = s[i:]
			continue
		}
		matched := false
		for _, p := range puncts {
			if strings.HasPrefix(s, p) {
				toks = append(toks, token{tokPunct, p})
				s = s[len(p):]
				matched = true
				break
			}
		}
		if !matched {
			return nil, &refusal{kind: refusedSyntax, token: string(r)}
		}
	}
	return append(toks, token{tokEOF, ""}), nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// lexString reads a quoted string opened by quote, unescaping it.
func lexString(s string, quote rune) (text, rest string, ok bool) {
	var b strings.Builder
	for i := 1; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch {
		case r == quote:
			return b.String(), s[i:], true
		case r == '\\' && i < len(s):
			e, n := utf8.DecodeRuneInString(s[i:])
			i += n
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteRune(e)
			}
		case r == '\n':
			return "", "", false
		default:
			b.WriteRune(r)
		}
	}
	return "", "", false
}

// lexNumber reads a decimal number with an optional fraction and exponent, or
// a hexadecimal one, which the parser then refuses.
func lexNumber(s string) (text, rest string) {
	i := 0
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		i = 2
		for i < len(s) && (isDigit(s[i]) || strings.IndexByte("abcdefABCDEF", s[i]) >= 0) {
			i++
		}
		return s[:i], s[i:]
	}
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && isDigit(s[j]) {
			for j < len(s) && isDigit(s[j]) {
				j++
			}
			i = j
		}
	}
	return s[:i], s[i:]
}

// opaqueParser reads a token stream as the subset's statements and expressions,
// writing v2 as it goes.
type opaqueParser struct {
	toks    []token
	i       int
	d       dialect
	sc      opaqueScope
	locals  map[string]string // names a `var` declared, with the scalar each holds
	assigns bool              // whether `=` assigns (a statement) rather than compares
}

func newOpaqueParser(body string, d dialect, sc opaqueScope) (*opaqueParser, *refusal) {
	toks, err := lexOpaque(body)
	if err != nil {
		return nil, err
	}
	return &opaqueParser{toks: toks, d: d, sc: sc, locals: map[string]string{}}, nil
}

// peek returns the next token, skipping newlines when skipNL is set.
func (p *opaqueParser) peek(skipNL bool) token {
	j := p.i
	for skipNL && p.toks[j].kind == tokNewline {
		j++
	}
	return p.toks[j]
}

// next consumes and returns the next token, skipping newlines when skipNL is set.
func (p *opaqueParser) next(skipNL bool) token {
	for skipNL && p.toks[p.i].kind == tokNewline {
		p.i++
	}
	t := p.toks[p.i]
	if t.kind != tokEOF {
		p.i++
	}
	return t
}

// isPunct reports whether t is the punctuation text.
func (t token) isPunct(text string) bool { return t.kind == tokPunct && t.text == text }

// word reports whether t is the identifier text, ignoring case.
func (t token) word(text string) bool {
	return t.kind == tokIdent && strings.EqualFold(t.text, text)
}

// scriptReserved lists the JavaScript words that open constructs the subset leaves out.
var scriptReserved = map[string]bool{
	"if": true, "else": true, "for": true, "while": true, "do": true, "function": true, "return": true,
	"switch": true, "case": true, "break": true, "continue": true, "throw": true, "try": true, "catch": true,
	"finally": true, "class": true, "new": true, "typeof": true, "delete": true, "instanceof": true, "in": true,
	"void": true, "with": true, "yield": true, "import": true, "export": true, "null": true, "undefined": true,
	"NaN": true, "Infinity": true,
}

// wholeExpr reads the body as one expression, which must reach its end.
func (p *opaqueParser) wholeExpr() (translated, *refusal) {
	t, err := p.expr()
	if err != nil {
		return translated{}, err
	}
	if tok := p.peek(true); tok.kind != tokEOF && !tok.isPunct(";") {
		return translated{}, &refusal{kind: refusedSyntax, token: tok.text, why: "text follows the expression"}
	}
	return t, nil
}

// wholeName reads an English body that is one property name, spaces and all.
func (p *opaqueParser) wholeName(body string) (translated, bool) {
	name := strings.TrimSpace(body)
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != ' ' {
			return translated{}, false
		}
	}
	if name == "" || !strings.Contains(name, " ") {
		return translated{}, false
	}
	ref, err := p.sc.feature([]string{name}, false)
	if err != nil {
		return translated{}, false
	}
	return translated{expr: ref.expr, scalar: ref.scalar, plural: ref.plural, atomic: true}, true
}

// statements reads the body as script statements, each written as v2 lines.
func (p *opaqueParser) statements() ([]string, *refusal) {
	p.assigns = true
	var lines []string
	for {
		tok := p.peek(true)
		if tok.isPunct(";") {
			p.next(true)
			continue
		}
		if tok.kind == tokEOF {
			break
		}
		written, err := p.statement()
		if err != nil {
			return nil, err
		}
		lines = append(lines, written...)
		switch end := p.peek(false); {
		case end.kind == tokEOF, end.isPunct(";"), end.kind == tokNewline:
			p.next(false)
		default:
			return nil, &refusal{kind: refusedSyntax, token: end.text, why: "a statement ends at `;` or a newline"}
		}
	}
	if len(lines) == 0 {
		return nil, &refusal{kind: refusedSyntax, token: "", why: "the body has no statements"}
	}
	return lines, nil
}

// statement reads one script statement: a simple declaration or an assignment.
func (p *opaqueParser) statement() ([]string, *refusal) {
	tok := p.peek(true)
	switch {
	case tok.kind == tokIdent && (tok.text == "var" || tok.text == "let" || tok.text == "const"):
		return p.declaration()
	case tok.kind == tokIdent && scriptReserved[tok.text]:
		return nil, &refusal{kind: refusedConstruct, token: tok.text}
	case tok.isPunct("{"):
		return nil, &refusal{kind: refusedConstruct, token: "{", why: "a block is not a statement of the subset"}
	case tok.isPunct("++") || tok.isPunct("--"):
		p.next(true)
		path, err := p.path()
		if err != nil {
			return nil, err
		}
		return p.step(path, tok.text)
	case tok.kind == tokIdent:
		path, err := p.path()
		if err != nil {
			return nil, err
		}
		op := p.peek(false)
		switch {
		case op.isPunct("="), op.isPunct("+="), op.isPunct("-="), op.isPunct("*="), op.isPunct("/="), op.isPunct("%="):
			p.next(false)
			return p.assignment(path, op.text)
		case op.isPunct("++"), op.isPunct("--"):
			p.next(false)
			return p.step(path, op.text)
		case op.isPunct("("):
			return nil, &refusal{kind: refusedCall, token: strings.Join(path, "."), why: "a call is not a statement of the subset"}
		}
		return nil, &refusal{kind: refusedConstruct, token: strings.Join(path, "."), why: "an expression that assigns nothing is not a statement of the subset"}
	}
	return nil, &refusal{kind: refusedSyntax, token: tok.text, why: "a statement starts with a name"}
}

// declaration reads `var x = e` as a local attribute of the action assigned e.
func (p *opaqueParser) declaration() ([]string, *refusal) {
	kw := p.next(true)
	name := p.next(true)
	if name.kind != tokIdent {
		return nil, &refusal{kind: refusedSyntax, token: name.text, why: kw.text + " declares a name"}
	}
	if !p.next(true).isPunct("=") {
		return nil, &refusal{kind: refusedConstruct, token: kw.text + " " + name.text, why: "only a declaration of one name with an initial value is translated"}
	}
	value, err := p.expr()
	if err != nil {
		return nil, err
	}
	if p.peek(false).isPunct(",") {
		return nil, &refusal{kind: refusedConstruct, token: kw.text + " " + name.text, why: "only a declaration of one name is translated"}
	}
	if value.scalar == "" || value.plural {
		return nil, &refusal{kind: refusedType, token: kw.text + " " + name.text, why: "the type the declaration holds cannot be told from its value"}
	}
	p.locals[name.text] = value.scalar
	target := writeName(name.text)
	return []string{
		"attribute " + target + " : " + value.scalar + ";",
		"assign " + target + " := " + value.expr + ";",
	}, nil
}

// step writes `x++` or `x--` as an assignment.
func (p *opaqueParser) step(path []string, op string) ([]string, *refusal) {
	target, err := p.target(path)
	if err != nil {
		return nil, err
	}
	if target.scalar != "" && !isNumeric(target.scalar) {
		return nil, &refusal{kind: refusedType, token: strings.Join(path, ".") + op, why: "a " + target.scalar + " is not counted"}
	}
	return []string{"assign " + target.expr + " := " + target.expr + " " + op[:1] + " 1;"}, nil
}

// assignment writes `x = e` or `x op= e` as an assignment.
func (p *opaqueParser) assignment(path []string, op string) ([]string, *refusal) {
	target, err := p.target(path)
	if err != nil {
		return nil, err
	}
	value, err := p.expr()
	if err != nil {
		return nil, err
	}
	name := strings.Join(path, ".")
	if op != "=" {
		if (target.scalar != "" && !isNumeric(target.scalar)) || (value.scalar != "" && !isNumeric(value.scalar)) {
			return nil, &refusal{kind: refusedType, token: name + " " + op, why: "the operation needs numbers"}
		}
		value = translated{expr: target.expr + " " + op[:1] + " " + value.operand(), scalar: arithmeticScalar(op[:1], target.scalar, value.scalar)}
	}
	if value.plural != target.plural {
		return nil, &refusal{kind: refusedType, token: name + " " + op, why: "one side is a collection and the other a single value"}
	}
	if target.scalar != "" && value.scalar != "" && !assignableScalar(target.scalar, value) {
		return nil, &refusal{kind: refusedType, token: name + " " + op, why: "a " + value.scalar + " is assigned to the " + target.scalar + " " + name + " holds"}
	}
	return []string{"assign " + target.expr + " := " + spellFor(target.scalar, value) + ";"}, nil
}

// target resolves the feature an assignment writes.
func (p *opaqueParser) target(path []string) (opaqueRef, *refusal) {
	if len(path) == 1 {
		if scalar, ok := p.locals[path[0]]; ok {
			return opaqueRef{expr: writeName(path[0]), scalar: scalar}, nil
		}
	}
	return p.sc.feature(path, true)
}

// path reads a dotted name.
func (p *opaqueParser) path() ([]string, *refusal) {
	first := p.next(true)
	if first.kind != tokIdent {
		return nil, &refusal{kind: refusedSyntax, token: first.text, why: "a name is expected"}
	}
	path := []string{first.text}
	for p.peek(false).isPunct(".") {
		p.next(false)
		step := p.next(true)
		if step.kind != tokIdent {
			return nil, &refusal{kind: refusedSyntax, token: step.text, why: "a member name follows `.`"}
		}
		path = append(path, step.text)
	}
	return path, nil
}

// expr reads a conditional expression, the top of the operator ladder.
func (p *opaqueParser) expr() (translated, *refusal) {
	cond, err := p.or()
	if err != nil {
		return translated{}, err
	}
	if !p.peek(true).isPunct("?") {
		return cond, nil
	}
	p.next(true)
	if cond.scalar != "" && cond.scalar != "Boolean" {
		return translated{}, &refusal{kind: refusedType, token: "?", why: "the condition is a " + cond.scalar + ", not a Boolean"}
	}
	yes, err := p.expr()
	if err != nil {
		return translated{}, err
	}
	if !p.next(true).isPunct(":") {
		return translated{}, &refusal{kind: refusedSyntax, token: "?", why: "`:` is expected after the first branch"}
	}
	no, err := p.expr()
	if err != nil {
		return translated{}, err
	}
	scalar, ok := commonScalar(yes, no)
	if !ok {
		return translated{}, &refusal{kind: refusedType, token: "?", why: "the branches are a " + yes.scalar + " and a " + no.scalar}
	}
	return translated{expr: "if " + cond.operand() + " ? " + yes.operand() + " else " + no.operand(), scalar: scalar, plural: yes.plural}, nil
}

// binaryOp is an operator of the ladder: how it is spelled in the source and in v2.
type binaryOp struct{ src, v2 string }

// or reads `a || b`, in English `a or b`.
func (p *opaqueParser) or() (translated, *refusal) {
	return p.logical(p.and, binaryOp{"||", "or"}, "or")
}

// and reads `a && b`, in English `a and b`.
func (p *opaqueParser) and() (translated, *refusal) {
	return p.logical(p.equality, binaryOp{"&&", "and"}, "and")
}

// logical reads a chain of one Boolean connective over operands read by next.
func (p *opaqueParser) logical(next func() (translated, *refusal), op binaryOp, word string) (translated, *refusal) {
	left, err := next()
	if err != nil {
		return translated{}, err
	}
	for {
		tok := p.peek(true)
		if !tok.isPunct(op.src) && !(p.d == dialectEnglish && tok.word(word)) {
			return left, nil
		}
		p.next(true)
		right, err := next()
		if err != nil {
			return translated{}, err
		}
		for _, side := range []translated{left, right} {
			if side.scalar != "" && side.scalar != "Boolean" {
				return translated{}, &refusal{kind: refusedType, token: tok.text, why: "an operand is a " + side.scalar + ", not a Boolean"}
			}
		}
		left = translated{expr: left.operand() + " " + op.v2 + " " + right.operand(), scalar: "Boolean"}
	}
}

// equality reads `a == b`, `a != b` and their strict forms; `=` compares
// where nothing assigns.
func (p *opaqueParser) equality() (translated, *refusal) {
	left, err := p.relational()
	if err != nil {
		return translated{}, err
	}
	for {
		tok := p.peek(true)
		var v2 string
		switch {
		case tok.isPunct("=="), tok.isPunct("==="), tok.isPunct("=") && !p.assigns:
			v2 = "=="
		case tok.isPunct("!="), tok.isPunct("!=="):
			v2 = "!="
		default:
			return left, nil
		}
		p.next(true)
		right, err := p.relational()
		if err != nil {
			return translated{}, err
		}
		if _, ok := commonScalar(left, right); !ok {
			return translated{}, &refusal{kind: refusedType, token: tok.text, why: "a " + left.scalar + " is compared with a " + right.scalar}
		}
		left = translated{expr: left.operand() + " " + v2 + " " + spellFor(left.scalar, right), scalar: "Boolean"}
	}
}

// relational reads `a < b`, `<=`, `>`, `>=` over numbers.
func (p *opaqueParser) relational() (translated, *refusal) {
	left, err := p.additive()
	if err != nil {
		return translated{}, err
	}
	for {
		tok := p.peek(true)
		if !tok.isPunct("<") && !tok.isPunct("<=") && !tok.isPunct(">") && !tok.isPunct(">=") {
			return left, nil
		}
		p.next(true)
		right, err := p.additive()
		if err != nil {
			return translated{}, err
		}
		if err := numbersAt(tok.text, left, right); err != nil {
			return translated{}, err
		}
		left = translated{expr: left.operand() + " " + tok.text + " " + right.operand(), scalar: "Boolean"}
	}
}

// additive reads `a + b` and `a - b` over numbers.
func (p *opaqueParser) additive() (translated, *refusal) {
	return p.arithmetic(p.multiplicative, "+", "-")
}

// multiplicative reads `a * b`, `a / b` and `a % b` over numbers.
func (p *opaqueParser) multiplicative() (translated, *refusal) {
	return p.arithmetic(p.power, "*", "/", "%")
}

// power reads `a ** b`, which binds tighter than the unary operators do in v2.
func (p *opaqueParser) power() (translated, *refusal) {
	left, err := p.unary()
	if err != nil {
		return translated{}, err
	}
	if !p.peek(true).isPunct("**") {
		return left, nil
	}
	p.next(true)
	right, err := p.power()
	if err != nil {
		return translated{}, err
	}
	if err := numbersAt("**", left, right); err != nil {
		return translated{}, err
	}
	return translated{expr: left.operand() + " ** " + right.operand(), scalar: "Real"}, nil
}

// arithmetic reads a chain of the given operators over operands read by next.
func (p *opaqueParser) arithmetic(next func() (translated, *refusal), ops ...string) (translated, *refusal) {
	left, err := next()
	if err != nil {
		return translated{}, err
	}
	for {
		tok := p.peek(true)
		op := ""
		for _, o := range ops {
			if tok.isPunct(o) {
				op = o
			}
		}
		if op == "" {
			return left, nil
		}
		p.next(true)
		right, err := next()
		if err != nil {
			return translated{}, err
		}
		if op == "+" && (left.scalar == "String" || right.scalar == "String") {
			return translated{}, &refusal{kind: refusedConstruct, token: "+", why: "string concatenation has no v2 form in the subset"}
		}
		if err := numbersAt(op, left, right); err != nil {
			return translated{}, err
		}
		left = translated{expr: left.operand() + " " + op + " " + right.operand(), scalar: arithmeticScalar(op, left.scalar, right.scalar)}
	}
}

// unary reads `-x`, `+x`, `!x` and in English `not x`.
func (p *opaqueParser) unary() (translated, *refusal) {
	tok := p.peek(true)
	switch {
	case tok.isPunct("-"), tok.isPunct("+"):
		p.next(true)
		x, err := p.unary()
		if err != nil {
			return translated{}, err
		}
		if x.scalar != "" && !isNumeric(x.scalar) {
			return translated{}, &refusal{kind: refusedType, token: tok.text, why: "the operand is a " + x.scalar + ", not a number"}
		}
		if tok.text == "+" {
			return x, nil
		}
		if x.lit == "integer" || x.lit == "real" {
			return translated{expr: "-" + x.expr, scalar: x.scalar, atomic: true, lit: x.lit}, nil
		}
		return translated{expr: "-" + x.operand(), scalar: x.scalar}, nil
	case tok.isPunct("!"), p.d == dialectEnglish && tok.word("not"):
		p.next(true)
		x, err := p.unary()
		if err != nil {
			return translated{}, err
		}
		if x.scalar != "" && x.scalar != "Boolean" {
			return translated{}, &refusal{kind: refusedType, token: tok.text, why: "the operand is a " + x.scalar + ", not a Boolean"}
		}
		return translated{expr: "not " + x.operand(), scalar: "Boolean"}, nil
	case tok.isPunct("++"), tok.isPunct("--"):
		return translated{}, &refusal{kind: refusedConstruct, token: tok.text, why: "counting inside an expression has no v2 form"}
	}
	return p.postfix()
}

// postfix reads a primary and what follows it: indexing and counting are refused.
func (p *opaqueParser) postfix() (translated, *refusal) {
	x, err := p.primary()
	if err != nil {
		return translated{}, err
	}
	switch tok := p.peek(false); {
	case tok.isPunct("["):
		return translated{}, &refusal{kind: refusedConstruct, token: "[", why: "indexing has no v2 form in the subset"}
	case tok.isPunct("++"), tok.isPunct("--"):
		return translated{}, &refusal{kind: refusedConstruct, token: tok.text, why: "counting inside an expression has no v2 form"}
	case tok.isPunct("("):
		return translated{}, &refusal{kind: refusedCall, token: x.expr, why: "only a named function is called"}
	}
	return x, nil
}

// primary reads a literal, a name (or a call of one) or a parenthesized expression.
func (p *opaqueParser) primary() (translated, *refusal) {
	tok := p.next(true)
	switch tok.kind {
	case tokNumber:
		return numberLiteral(tok.text)
	case tokString:
		return translated{expr: stringLiteral(tok.text), scalar: "String", atomic: true, lit: "string"}, nil
	case tokIdent:
		switch {
		case tok.word("true") || tok.word("false"):
			if p.d == dialectScript && tok.text != strings.ToLower(tok.text) {
				return translated{}, &refusal{kind: refusedName, token: tok.text, why: "a script spells its Booleans in lower case"}
			}
			return translated{expr: strings.ToLower(tok.text), scalar: "Boolean", atomic: true, lit: "boolean"}, nil
		case p.d == dialectScript && scriptReserved[tok.text]:
			return translated{}, &refusal{kind: refusedConstruct, token: tok.text}
		}
		p.i--
		path, err := p.path()
		if err != nil {
			return translated{}, err
		}
		if p.peek(false).isPunct("(") {
			p.next(false)
			return p.call(path)
		}
		return p.name(path)
	case tokPunct:
		if tok.text == "(" {
			x, err := p.expr()
			if err != nil {
				return translated{}, err
			}
			if !p.next(true).isPunct(")") {
				return translated{}, &refusal{kind: refusedSyntax, token: "(", why: "the parenthesis is not closed"}
			}
			if !x.atomic {
				x.expr = "(" + x.expr + ")"
				x.atomic = true
			}
			return x, nil
		}
		if tok.text == "{" || tok.text == "[" {
			return translated{}, &refusal{kind: refusedConstruct, token: tok.text, why: "an object or array literal has no v2 form in the subset"}
		}
		if tok.text == "/" {
			return translated{}, &refusal{kind: refusedConstruct, token: "/", why: "a regular expression has no v2 form"}
		}
		return translated{}, &refusal{kind: refusedSyntax, token: tok.text, why: "an operand is expected"}
	case tokNewline:
		return translated{}, &refusal{kind: refusedSyntax, token: "\n", why: "an operand is expected"}
	}
	return translated{}, &refusal{kind: refusedSyntax, token: "", why: "the expression ends early"}
}

// name resolves a dotted name through the locals and the scope.
func (p *opaqueParser) name(path []string) (translated, *refusal) {
	if len(path) == 1 {
		if scalar, ok := p.locals[path[0]]; ok {
			return translated{expr: writeName(path[0]), scalar: scalar, atomic: true}, nil
		}
	}
	ref, err := p.sc.feature(path, false)
	if err != nil {
		return translated{}, err
	}
	return translated{expr: ref.expr, scalar: ref.scalar, plural: ref.plural, atomic: true}, nil
}

// call reads the arguments of a call and writes the library function the table maps it to.
func (p *opaqueParser) call(path []string) (translated, *refusal) {
	fn := strings.Join(path, ".")
	var args []translated
	if !p.peek(true).isPunct(")") {
		for {
			arg, err := p.expr()
			if err != nil {
				return translated{}, err
			}
			args = append(args, arg)
			sep := p.next(true)
			if sep.isPunct(")") {
				break
			}
			if !sep.isPunct(",") {
				return translated{}, &refusal{kind: refusedSyntax, token: fn + "(", why: "the arguments are not closed"}
			}
		}
	} else {
		p.next(true)
	}
	arity := func(n int) *refusal {
		if len(args) != n {
			return &refusal{kind: refusedCall, token: fn, why: fn + " takes " + strconv.Itoa(n) + " argument(s), not " + strconv.Itoa(len(args))}
		}
		return nil
	}
	switch fn {
	case "Math.max", "Math.min":
		if len(args) < 2 {
			return translated{}, &refusal{kind: refusedCall, token: fn, why: fn + " takes at least 2 arguments"}
		}
		acc := args[0]
		for _, arg := range args[1:] {
			if err := numbersAt(fn, acc, arg); err != nil {
				return translated{}, err
			}
			scalar := arithmeticScalar("+", acc.scalar, arg.scalar)
			acc = translated{expr: extremum(fn[5:], scalar) + "(" + acc.expr + ", " + arg.expr + ")", scalar: scalar, atomic: true}
		}
		return acc, nil
	case "Math.abs":
		if err := arity(1); err != nil {
			return translated{}, err
		}
		if err := numbersAt(fn, args[0], args[0]); err != nil {
			return translated{}, err
		}
		lib := "NumericalFunctions"
		switch {
		case wholeScalar(args[0].scalar):
			lib = "IntegerFunctions"
		case args[0].scalar != "":
			lib = "RealFunctions"
		}
		return translated{expr: lib + "::abs(" + args[0].expr + ")", scalar: args[0].scalar, atomic: true}, nil
	case "Math.floor", "Math.round", "Math.ceil":
		if err := arity(1); err != nil {
			return translated{}, err
		}
		if err := numbersAt(fn, args[0], args[0]); err != nil {
			return translated{}, err
		}
		switch fn {
		case "Math.ceil":
			return translated{expr: "-RealFunctions::floor(-" + args[0].operand() + ")", scalar: "Integer"}, nil
		default:
			return translated{expr: "RealFunctions::" + fn[5:] + "(" + args[0].expr + ")", scalar: "Integer", atomic: true}, nil
		}
	case "Math.sqrt":
		if err := arity(1); err != nil {
			return translated{}, err
		}
		if err := numbersAt(fn, args[0], args[0]); err != nil {
			return translated{}, err
		}
		return translated{expr: "RealFunctions::sqrt(" + args[0].expr + ")", scalar: "Real", atomic: true}, nil
	case "Math.pow":
		if err := arity(2); err != nil {
			return translated{}, err
		}
		if err := numbersAt(fn, args[0], args[1]); err != nil {
			return translated{}, err
		}
		return translated{expr: args[0].operand() + " ** " + args[1].operand(), scalar: "Real"}, nil
	case "java.util.Collections.max", "java.util.Collections.min", "Collections.max", "Collections.min":
		if err := arity(1); err != nil {
			return translated{}, err
		}
		s := args[0]
		if !s.plural {
			return translated{}, &refusal{kind: refusedType, token: fn, why: "the argument is a single value, not a collection"}
		}
		if s.scalar != "" && !isNumeric(s.scalar) {
			return translated{}, &refusal{kind: refusedType, token: fn, why: "the collection holds " + s.scalar + " values, not numbers"}
		}
		which := fn[strings.LastIndex(fn, ".")+1:]
		return translated{expr: s.operand() + "->ControlFunctions::reduce { in x; in y; " + extremum(which, s.scalar) + "(x, y) }", scalar: s.scalar, atomic: true}, nil
	}
	return translated{}, &refusal{kind: refusedCall, token: fn}
}

// extremum names the library max or min function for numbers of the given scalar.
func extremum(which, scalar string) string {
	switch {
	case wholeScalar(scalar):
		return "IntegerFunctions::" + which
	case scalar != "":
		return "RealFunctions::" + which
	}
	return "NumericalFunctions::" + which
}

// numberLiteral writes a script number as a v2 integer or real literal.
func numberLiteral(text string) (translated, *refusal) {
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		return translated{}, &refusal{kind: refusedConstruct, token: text, why: "a hexadecimal literal has no v2 form in the subset"}
	}
	if !strings.ContainsAny(text, ".eE") {
		return translated{expr: text, scalar: "Integer", atomic: true, lit: "integer"}, nil
	}
	v, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return translated{}, &refusal{kind: refusedSyntax, token: text, why: "the number is not finite"}
	}
	return translated{expr: realLiteral(v), scalar: "Real", atomic: true, lit: "real"}, nil
}

// stringLiteral writes text as a v2 string literal.
func stringLiteral(text string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`, "\r", `\r`)
	return `"` + r.Replace(text) + `"`
}

// numbersAt refuses operands of op that are known not to be numbers.
func numbersAt(op string, sides ...translated) *refusal {
	for _, s := range sides {
		if s.plural {
			return &refusal{kind: refusedType, token: op, why: "an operand is a collection, not a number"}
		}
		if s.scalar != "" && !isNumeric(s.scalar) {
			return &refusal{kind: refusedType, token: op, why: "an operand is a " + s.scalar + ", not a number"}
		}
	}
	return nil
}

// isNumeric reports whether the scalar is a kind of number.
func isNumeric(s string) bool { return numericScalar[s] }

// wholeScalar reports whether the scalar holds integers.
func wholeScalar(s string) bool {
	return s == "Integer" || s == "Natural"
}

// realScalar reports whether the scalar holds non-integer numbers.
func realScalar(s string) bool {
	return s == "Real" || s == "Rational" || s == "Number" || s == "Complex"
}

// arithmeticScalar is the scalar an arithmetic operator yields: integers stay
// integers except under `/`, which a script computes as a real.
func arithmeticScalar(op, a, b string) string {
	switch {
	case op == "/":
		return "Real"
	case wholeScalar(a) && wholeScalar(b):
		return "Integer"
	case realScalar(a) || realScalar(b):
		return "Real"
	}
	return ""
}

// commonScalar is the scalar two expressions share, or agree with as numbers;
// ok is false when their known types conflict.
func commonScalar(a, b translated) (string, bool) {
	switch {
	case a.plural != b.plural:
		return "", false
	case a.scalar == "":
		return b.scalar, true
	case b.scalar == "":
		return a.scalar, true
	case a.scalar == b.scalar:
		return a.scalar, true
	case isNumeric(a.scalar) && isNumeric(b.scalar):
		return arithmeticScalar("+", a.scalar, b.scalar), true
	}
	return "", false
}

// assignableScalar reports whether value may be held by a feature of scalar:
// the same type, or an integer where a real is held, or a whole real literal
// where an integer is.
func assignableScalar(scalar string, value translated) bool {
	switch {
	case value.scalar == "" || scalar == "" || value.scalar == scalar:
		return true
	case realScalar(scalar) && isNumeric(value.scalar):
		return true
	case wholeScalar(scalar) && value.lit == "real":
		_, ok := scalarLiteral("real", value.expr, value.expr, scalar)
		return ok
	}
	return false
}

// spellFor writes value as held by a feature of scalar: a whole real literal
// assigned to an integer loses its point.
func spellFor(scalar string, value translated) string {
	if wholeScalar(scalar) && value.lit == "real" {
		if spelled, ok := scalarLiteral("real", value.expr, value.expr, scalar); ok {
			return spelled
		}
	}
	return value.expr
}
