package migrate

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
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

// final reports whether the refusal settles the body: a body in a language the
// translator reads is what that language says, so text it refuses is not then
// read as v2 syntax, which overlaps by coincidence. A body in a language the
// translator does not read, or in none, may still be v2 syntax.
func (r *refusal) final(lang string) bool {
	if strings.TrimSpace(lang) == "" {
		return false
	}
	return r.kind != refusedLanguage
}

// opaqueRef is what a scope answers for a name: the v2 expression reading it,
// the scalar it holds ("" when unknown or not a scalar), the non-scalar type
// it is known to hold followed by every type that generalizes it (nil when a
// scalar or unknown), plural for a collection.
type opaqueRef struct {
	expr   string
	scalar string
	object []string
	plural bool
}

// value is the ref read as an expression.
func (r opaqueRef) value() translated {
	return translated{expr: r.expr, scalar: r.scalar, object: r.object, plural: r.plural, atomic: true}
}

// opaqueScope answers what the names of an opaque body mean where it is read;
// a path starting with `this` asks for a feature of the context object.
type opaqueScope interface {
	feature(path []string, write bool) (opaqueRef, *refusal)
}

// translated is a v2 expression the translator produced, with what it knows
// of its type (scalar and object as on opaqueRef); atomic is true when it needs
// no parentheses as an operand, loose is the v2 precedence of its outermost
// operator when it is not atomic, and lit names the literal kind when the
// expression is one literal.
type translated struct {
	expr   string
	scalar string
	object []string
	plural bool
	atomic bool
	loose  int
	lit    string
}

// wanted is the type a translated expression must yield: what the feature
// holding it holds (scalar and object as on opaqueRef), one value when single;
// the zero value wants any value.
type wanted struct {
	scalar string
	object []string
	single bool
}

// oneOf wants one value of scalar ("" for any scalar or object).
func oneOf(scalar string) wanted {
	return wanted{scalar: scalar, single: true}
}

// target is the wanted type as the value a feature of that type reads.
func (w wanted) target() translated {
	return translated{scalar: w.scalar, object: w.object}
}

// held names what t is known to hold, for a refusal: its scalar, its
// non-scalar type, or "" when nothing is known.
func (t translated) held() string {
	if t.scalar != "" {
		return t.scalar
	}
	if len(t.object) > 0 {
		return t.object[0]
	}
	return ""
}

// The v2 operators the translator writes, loosest last: an operand of a looser
// operator is parenthesized.
const (
	looseUnary = iota + 1
	loosePower
	looseMultiplicative
	looseAdditive
	looseRelational
	looseEquality
	looseAnd
	looseOr
	looseConditional
)

// operand writes t as the operand of an operator that binds least of all.
func (t translated) operand() string {
	if t.atomic {
		return t.expr
	}
	return "(" + t.expr + ")"
}

// operandOf writes t as the left or right operand of an operator of looseness
// loose. The binary operators associate left, but `**` to the right.
func (t translated) operandOf(loose int, right bool) string {
	switch {
	case t.atomic:
		return t.expr
	case t.loose == looseUnary && loose == loosePower && !right:
		// `(-x) ** y` binds so in v2 too, but the parentheses spell out what a script reader would doubt.
	case t.loose < loose:
		return t.expr
	case t.loose == loose && right == (loose == loosePower):
		return t.expr
	}
	return "(" + t.expr + ")"
}

// binary writes left op right at looseness loose.
func binary(left translated, op string, right translated, loose int, scalar string) translated {
	return translated{expr: left.operandOf(loose, false) + " " + op + " " + right.operandOf(loose, true), scalar: scalar, loose: loose}
}

// dialect is the family an opaque language belongs to.
type dialect int

const (
	dialectNone    dialect = iota // a language the translator does not read
	dialectScript                 // JavaScript, ECMAScript or no language
	dialectJava                   // Java, whose `/` of two whole numbers drops the remainder
	dialectEnglish                // English or natural-language text
)

// script reports whether the dialect is a script, read as statements and expressions.
func (d dialect) script() bool { return d == dialectScript || d == dialectJava }

// dialectOf classifies an opaque body's language.
func dialectOf(lang string) dialect {
	l := strings.ToLower(strings.TrimSpace(lang))
	switch {
	case l == "", strings.HasPrefix(l, "javascript"), strings.HasPrefix(l, "ecmascript"),
		l == "js", strings.HasPrefix(l, "rhino"), strings.HasPrefix(l, "nashorn"):
		return dialectScript
	case strings.HasPrefix(l, "java"):
		return dialectJava
	case l == "english", l == "natural", strings.HasPrefix(l, "natural language"), l == "text", l == "plain":
		return dialectEnglish
	}
	return dialectNone
}

// translateExpr translates body as one expression read in sc yielding what
// want asks for. The expression is complete or refused.
func translateExpr(body, lang string, sc opaqueScope, want wanted) (translated, *refusal) {
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
	if want.single && t.plural {
		return translated{}, &refusal{kind: refusedType, token: body,
			why: "the expression is a collection, not the one value wanted"}
	}
	if target := want.target(); target.held() != "" && t.held() != "" && !assignableTo(target, t) {
		return translated{}, &refusal{kind: refusedType, token: body,
			why: "the expression is a " + t.held() + ", not the " + target.held() + " wanted"}
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
	return p.wholeExpr()
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

// lineTerminator reports whether r ends a line as JavaScript reads it.
func lineTerminator(r rune) bool {
	return r == '\n' || r == '\r' || r == '\u2028' || r == '\u2029'
}

// lexOpaque scans body into tokens; comments are dropped and line ends kept,
// as a script ends a statement at one. A block comment spanning lines is one.
func lexOpaque(body string) ([]token, *refusal) {
	var toks []token
	s := body
	for s != "" {
		r, size := utf8.DecodeRuneInString(s)
		switch {
		case lineTerminator(r):
			if strings.HasPrefix(s, "\r\n") {
				size = 2
			}
			toks = append(toks, token{tokNewline, "\n"})
			s = s[size:]
			continue
		case unicode.IsSpace(r):
			s = s[size:]
			continue
		case strings.HasPrefix(s, "//"):
			if i := strings.IndexFunc(s, lineTerminator); i >= 0 {
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
			if strings.ContainsFunc(s[2:2+i], lineTerminator) {
				toks = append(toks, token{tokNewline, "\n"})
			}
			s = s[i+4:]
			continue
		case r == '"' || r == '\'':
			text, rest, err := lexString(s, r)
			if err != nil {
				return nil, err
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

// lexString reads a quoted string opened by quote, decoding JavaScript's escapes.
func lexString(s string, quote rune) (text, rest string, err *refusal) {
	unclosed := &refusal{kind: refusedSyntax, token: string(quote), why: "the string is not closed"}
	var b strings.Builder
	for i := 1; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch {
		case r == quote:
			return b.String(), s[i:], nil
		case r == '\\':
			e, n, err := unescape(s[i:])
			if err != nil {
				return "", "", err
			}
			i += n
			if e >= 0 {
				b.WriteRune(e)
			}
		case r == '\n' || r == '\r':
			return "", "", unclosed
		default:
			b.WriteRune(r)
		}
	}
	return "", "", unclosed
}

// unescape decodes the JavaScript escape whose backslash precedes s: the
// character it stands for (-1 for a line continuation) and the bytes read.
// Legacy octal escapes and characters the notation cannot spell are refused.
func unescape(s string) (rune, int, *refusal) {
	if s == "" {
		return 0, 0, &refusal{kind: refusedSyntax, token: `\`, why: "the string is not closed"}
	}
	e, n := utf8.DecodeRuneInString(s)
	var r rune
	switch e {
	case 'n':
		r = '\n'
	case 't':
		r = '\t'
	case 'r':
		r = '\r'
	case 'b':
		r = '\b'
	case 'f':
		r = '\f'
	case 'v':
		r = '\v'
	case '0':
		if n < len(s) && isDigit(s[n]) {
			return 0, 0, &refusal{kind: refusedConstruct, token: `\` + s[:n+1], why: "a legacy octal escape"}
		}
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return 0, 0, &refusal{kind: refusedConstruct, token: `\` + s[:n], why: "a legacy octal escape"}
	case 'x':
		hex := hexRun(s[1:], 2)
		if len(hex) != 2 {
			return 0, 0, &refusal{kind: refusedSyntax, token: `\x` + hex, why: "a hexadecimal escape needs two hex digits"}
		}
		v, _ := strconv.ParseUint(hex, 16, 32)
		r, n = rune(v), 3
	case 'u':
		var err *refusal
		if r, n, err = unicodeEscape(s); err != nil {
			return 0, 0, err
		}
		// A high surrogate joined by a low one is a single character in UTF-16.
		if utf16.IsSurrogate(r) && r < 0xDC00 && strings.HasPrefix(s[n:], `\u`) {
			if lo, m, err := unicodeEscape(s[n+1:]); err == nil && utf16.IsSurrogate(lo) && lo >= 0xDC00 {
				r, n = utf16.DecodeRune(r, lo), n+1+m
			}
		}
	case '\n':
		return -1, n, nil
	case '\r':
		if strings.HasPrefix(s, "\r\n") {
			n = 2
		}
		return -1, n, nil
	default:
		r = e
	}
	if spelled, ok := spelledInNotation(r); !ok {
		return 0, 0, &refusal{kind: refusedConstruct, token: `\` + s[:n], why: "the notation spells no " + spelled}
	}
	return r, n, nil
}

// unicodeEscape reads the `\uHHHH` or `\u{H…}` escape whose backslash precedes s:
// the code unit or point it names and the bytes read.
func unicodeEscape(s string) (rune, int, *refusal) {
	hex, width := hexRun(s[1:], 4), 5
	if strings.HasPrefix(s, "u{") {
		hex = hexRun(s[2:], len(s))
		width = 3 + len(hex)
		if !strings.HasPrefix(s[2+len(hex):], "}") {
			return 0, 0, &refusal{kind: refusedSyntax, token: `\u{` + hex, why: "a Unicode escape's brace is not closed"}
		}
	} else if len(hex) != 4 {
		return 0, 0, &refusal{kind: refusedSyntax, token: `\u` + hex, why: "a Unicode escape needs four hex digits"}
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil || v > unicode.MaxRune {
		return 0, 0, &refusal{kind: refusedSyntax, token: `\` + s[:width], why: "a Unicode escape names no character"}
	}
	return rune(v), width, nil
}

func isHex(c byte) bool { return isDigit(c) || strings.IndexByte("abcdefABCDEF", c) >= 0 }

// hexRun is the run of at most limit hex digits opening s.
func hexRun(s string, limit int) string {
	i := 0
	for i < len(s) && i < limit && isHex(s[i]) {
		i++
	}
	return s[:i]
}

// spelledInNotation reports whether a string value may hold r: KerML escapes
// only \b \t \n \f \r, so other control characters have no spelling.
func spelledInNotation(r rune) (string, bool) {
	switch {
	case r == 0:
		return "NUL character", false
	case r == '\v':
		return "vertical tab", false
	case r == 0x7f || (r < 0x20 && !strings.ContainsRune("\b\t\n\f\r", r)):
		return "control character U+" + strings.ToUpper(strconv.FormatInt(int64(r), 16)), false
	case utf16Surrogate(r):
		return "lone surrogate", false
	}
	return "", true
}

func utf16Surrogate(r rune) bool { return r >= 0xD800 && r <= 0xDFFF }

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
	locals  map[string]local // names a `var`, `let` or `const` declared
	assigns bool             // whether `=` assigns (a statement) rather than compares
}

func newOpaqueParser(body string, d dialect, sc opaqueScope) (*opaqueParser, *refusal) {
	toks, err := lexOpaque(body)
	if err != nil {
		return nil, err
	}
	return &opaqueParser{toks: toks, d: d, sc: sc, locals: map[string]local{}}, nil
}

// local is a name a declaration introduced: the scalar it holds and whether
// `const` made it unassignable.
type local struct {
	scalar   string
	constant bool
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
	if p.peek(true).isPunct(";") {
		p.next(true)
	}
	if tok := p.peek(true); tok.kind != tokEOF {
		return translated{}, &refusal{kind: refusedSyntax, token: tok.text, why: "text follows the expression"}
	}
	return t, nil
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
	if err := p.declarable(kw.text, name.text); err != nil {
		return nil, err
	}
	value, err := p.expr()
	if err != nil {
		return nil, err
	}
	if p.peek(false).isPunct(",") {
		return nil, &refusal{kind: refusedConstruct, token: kw.text + " " + name.text, why: "only a declaration of one name is translated"}
	}
	if value.scalar == "" || value.plural {
		why := "the type the declaration holds cannot be told from its value"
		if value.held() != "" {
			why = "the value is a " + value.held() + ", not a scalar a local attribute holds"
		}
		return nil, &refusal{kind: refusedType, token: kw.text + " " + name.text, why: why}
	}
	p.locals[name.text] = local{scalar: value.scalar, constant: kw.text == "const"}
	target := writeName(name.text)
	return []string{
		"attribute " + target + " : ScalarValues::" + value.scalar + ";",
		"assign " + target + " := " + value.expr + ";",
	}, nil
}

// declarable refuses a declaration whose name the action body already has: a
// local declared before, a member every action inherits, or a feature the
// scope reads by that name, which a second declaration would make ambiguous.
func (p *opaqueParser) declarable(kw, name string) *refusal {
	token := kw + " " + name
	if _, ok := p.locals[name]; ok {
		return &refusal{kind: refusedConstruct, token: token, why: name + " is declared again"}
	}
	if inheritedActionNames()[name] {
		return &refusal{kind: refusedConstruct, token: token, why: name + " is a member every action has"}
	}
	if _, any := p.sc.(anyScope); any {
		return nil
	}
	if _, err := p.sc.feature([]string{name}, false); err == nil {
		return &refusal{kind: refusedConstruct, token: token, why: name + " is already a feature here, which a declaration would shadow"}
	}
	return nil
}

// step writes `x++` or `x--` as an assignment.
func (p *opaqueParser) step(path []string, op string) ([]string, *refusal) {
	target, err := p.target(path)
	if err != nil {
		return nil, err
	}
	if held := target.value().held(); held != "" && !isNumeric(target.scalar) {
		return nil, &refusal{kind: refusedType, token: strings.Join(path, ".") + op, why: "a " + held + " is not counted"}
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
	held := target.value()
	if op != "=" {
		if err := numbersAt(name+" "+op, held, value); err != nil {
			return nil, err
		}
		if value, err = p.arithmeticOf(held, op[:1], value); err != nil {
			return nil, err
		}
	}
	if value.plural != target.plural {
		return nil, &refusal{kind: refusedType, token: name + " " + op, why: "one side is a collection and the other a single value"}
	}
	if held.held() != "" && value.held() != "" && !assignableTo(held, value) {
		return nil, &refusal{kind: refusedType, token: name + " " + op, why: "a " + value.held() + " is assigned to the " + held.held() + " " + name + " holds"}
	}
	return []string{"assign " + target.expr + " := " + spellFor(target.scalar, value) + ";"}, nil
}

// target resolves the feature an assignment writes.
func (p *opaqueParser) target(path []string) (opaqueRef, *refusal) {
	if len(path) == 1 {
		if l, ok := p.locals[path[0]]; ok {
			if l.constant {
				return opaqueRef{}, &refusal{kind: refusedConstruct, token: path[0], why: "a const is not assigned again"}
			}
			return opaqueRef{expr: writeName(path[0]), scalar: l.scalar}, nil
		}
	}
	return p.sc.feature(path, true)
}

// englishWords are the words an English guard uses as operators, never as part of a name.
var englishWords = map[string]bool{"and": true, "or": true, "not": true}

// path reads a dotted name; in English a name may be several words (`Ready To Go`).
func (p *opaqueParser) path() ([]string, *refusal) {
	first := p.next(true)
	if first.kind != tokIdent {
		return nil, &refusal{kind: refusedSyntax, token: first.text, why: "a name is expected"}
	}
	path := []string{first.text}
	if p.d == dialectEnglish && !p.peek(false).isPunct(".") {
		name, err := p.spacedName(first.text)
		if err != nil {
			return nil, err
		}
		path[0] = name
	}
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

// spacedName extends an English name with the words that follow it up to the next
// operator word: English juxtaposes nothing, so the run is one name or no name.
func (p *opaqueParser) spacedName(first string) (string, *refusal) {
	words := []string{first}
	for j := p.i; ; j++ {
		tok := p.toks[j]
		if (tok.kind != tokIdent && tok.kind != tokNumber) || englishWords[strings.ToLower(tok.text)] {
			break
		}
		words = append(words, tok.text)
	}
	if len(words) == 1 {
		return first, nil
	}
	name := strings.Join(words, " ")
	if _, err := p.sc.feature([]string{name}, false); err != nil {
		return "", err
	}
	p.i += len(words) - 1
	return name, nil
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
	if cond.held() != "" && cond.scalar != "Boolean" {
		return translated{}, &refusal{kind: refusedType, token: "?", why: "the condition is a " + cond.held() + ", not a Boolean"}
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
		return translated{}, &refusal{kind: refusedType, token: "?", why: "the branches are a " + yes.held() + " and a " + no.held()}
	}
	return translated{expr: "if " + cond.operand() + " ? " + yes.operand() + " else " + no.operand(), scalar: scalar, object: commonObject(yes, no), plural: yes.plural, loose: looseConditional}, nil
}

// binaryOp is an operator of the ladder: how it is spelled in the source and in v2.
type binaryOp struct{ src, v2 string }

// or reads `a || b`, in English `a or b`.
func (p *opaqueParser) or() (translated, *refusal) {
	return p.logical(p.and, binaryOp{"||", "or"}, "or", looseOr)
}

// and reads `a && b`, in English `a and b`.
func (p *opaqueParser) and() (translated, *refusal) {
	return p.logical(p.equality, binaryOp{"&&", "and"}, "and", looseAnd)
}

// logical reads a chain of one Boolean connective over operands read by next.
func (p *opaqueParser) logical(next func() (translated, *refusal), op binaryOp, word string, loose int) (translated, *refusal) {
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
			if side.held() != "" && side.scalar != "Boolean" {
				return translated{}, &refusal{kind: refusedType, token: tok.text, why: "an operand is a " + side.held() + ", not a Boolean"}
			}
		}
		left = binary(left, op.v2, right, loose, "Boolean")
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
			return translated{}, &refusal{kind: refusedType, token: tok.text, why: "a " + left.held() + " is compared with a " + right.held()}
		}
		right.expr = spellFor(left.scalar, right)
		left = binary(left, v2, right, looseEquality, "Boolean")
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
		left = binary(left, tok.text, right, looseRelational, "Boolean")
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
// JavaScript has no `-a ** b`: a unary operand of `**` must be parenthesized,
// so the unparenthesized form is refused rather than read as v2's `-(a ** b)`.
func (p *opaqueParser) power() (translated, *refusal) {
	prefix := p.peek(true)
	left, err := p.unary()
	if err != nil {
		return translated{}, err
	}
	if !p.peek(true).isPunct("**") {
		return left, nil
	}
	if p.d == dialectJava {
		return translated{}, &refusal{kind: refusedConstruct, token: "**", why: "Java has no exponentiation operator"}
	}
	if p.unaryPrefix(prefix) {
		token := left.expr
		if prefix.text == "+" {
			token = "+" + token
		}
		return translated{}, &refusal{kind: refusedConstruct, token: token + " **",
			why: "JavaScript parenthesizes a unary operand of `**`"}
	}
	p.next(true)
	right, err := p.power()
	if err != nil {
		return translated{}, err
	}
	if err := numbersAt("**", left, right); err != nil {
		return translated{}, err
	}
	return binary(left, "**", right, loosePower, "Real"), nil
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
		if left, err = p.arithmeticOf(left, op, right); err != nil {
			return translated{}, err
		}
	}
}

// arithmeticOf writes left op right. Java's `/` of two whole numbers drops the
// remainder, so it is written as that quotient, or refused when the operands'
// types cannot tell whether it does.
func (p *opaqueParser) arithmeticOf(left translated, op string, right translated) (translated, *refusal) {
	if op == "/" && p.d == dialectJava {
		switch {
		case wholeScalar(left.scalar) && wholeScalar(right.scalar):
			return javaQuotient(left, right), nil
		case !realScalar(left.scalar) && !realScalar(right.scalar):
			side := left
			if side.scalar != "" {
				side = right
			}
			return translated{}, &refusal{kind: refusedType, token: "/",
				why: "Java divides two whole numbers without remainder, and whether " + side.expr + " holds one cannot be told"}
		}
	}
	return binary(left, op, right, arithmeticLoose(op), arithmeticScalar(op, left.scalar, right.scalar)), nil
}

// javaQuotient writes Java's `/` over whole numbers x and y: the exact quotient
// truncated toward zero, which the extension library's quotient computes and
// reports as overflow for the one pair (the least Integer by -1) outside the range.
func javaQuotient(x, y translated) translated {
	return translated{expr: "OpenSysMLMathFunctions::quotient(" + x.expr + ", " + y.expr + ")", scalar: "Integer", atomic: true}
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
		if x.held() != "" && !isNumeric(x.scalar) {
			return translated{}, &refusal{kind: refusedType, token: tok.text, why: "the operand is a " + x.held() + ", not a number"}
		}
		if tok.text == "+" {
			return x, nil
		}
		if x.lit == "integer" || x.lit == "real" {
			return translated{expr: "-" + x.expr, scalar: x.scalar, atomic: true, lit: x.lit}, nil
		}
		return translated{expr: "-" + x.operandOf(looseUnary, true), scalar: x.scalar, loose: looseUnary}, nil
	case tok.isPunct("!"), p.d == dialectEnglish && tok.word("not"):
		p.next(true)
		x, err := p.unary()
		if err != nil {
			return translated{}, err
		}
		if x.held() != "" && x.scalar != "Boolean" {
			return translated{}, &refusal{kind: refusedType, token: tok.text, why: "the operand is a " + x.held() + ", not a Boolean"}
		}
		return translated{expr: "not " + x.operandOf(looseUnary, true), scalar: "Boolean", loose: looseUnary}, nil
	case tok.isPunct("++"), tok.isPunct("--"):
		return translated{}, &refusal{kind: refusedConstruct, token: tok.text, why: "counting inside an expression has no v2 form"}
	}
	return p.postfix()
}

// unaryPrefix reports whether tok opens a unary expression of the dialect.
func (p *opaqueParser) unaryPrefix(tok token) bool {
	return tok.isPunct("-") || tok.isPunct("+") || tok.isPunct("!") || (p.d == dialectEnglish && tok.word("not"))
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
		return numberLiteral(tok.text, p.d)
	case tokString:
		return translated{expr: stringLiteral(tok.text), scalar: "String", atomic: true, lit: "string"}, nil
	case tokIdent:
		switch {
		case tok.word("true") || tok.word("false"):
			if p.d.script() && tok.text != strings.ToLower(tok.text) {
				return translated{}, &refusal{kind: refusedName, token: tok.text, why: "a script spells its Booleans in lower case"}
			}
			return translated{expr: strings.ToLower(tok.text), scalar: "Boolean", atomic: true, lit: "boolean"}, nil
		case p.d.script() && scriptReserved[tok.text]:
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
			// The group keeps its looseness: operands are re-parenthesized where the v2 precedence needs it.
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
		if l, ok := p.locals[path[0]]; ok {
			return translated{expr: writeName(path[0]), scalar: l.scalar, atomic: true}, nil
		}
	}
	ref, err := p.sc.feature(path, false)
	if err != nil {
		return translated{}, err
	}
	return ref.value(), nil
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
			return translated{expr: "-RealFunctions::floor(-" + args[0].operandOf(looseUnary, true) + ")", scalar: "Integer", loose: looseUnary}, nil
		case "Math.round":
			// JavaScript rounds a half toward +∞, where RealFunctions::round rounds it away from zero.
			half := translated{expr: "0.5", scalar: "Real", atomic: true, lit: "real"}
			return translated{expr: "RealFunctions::floor(" + binary(args[0], "+", half, looseAdditive, "Real").expr + ")", scalar: "Integer", atomic: true}, nil
		default:
			return translated{expr: "RealFunctions::floor(" + args[0].expr + ")", scalar: "Integer", atomic: true}, nil
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
		return binary(args[0], "**", args[1], loosePower, "Real"), nil
	case "java.util.Collections.max", "java.util.Collections.min", "Collections.max", "Collections.min":
		if err := arity(1); err != nil {
			return translated{}, err
		}
		s := args[0]
		if !s.plural {
			return translated{}, &refusal{kind: refusedType, token: fn, why: "the argument is a single value, not a collection"}
		}
		if s.held() != "" && !isNumeric(s.scalar) {
			return translated{}, &refusal{kind: refusedType, token: fn, why: "the collection holds " + s.held() + " values, not numbers"}
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

// maxSafeInteger is the largest whole number a script's floating-point Number
// holds exactly; a longer spelling is rounded as the script reads it.
const maxSafeInteger = 1<<53 - 1

// numberLiteral writes a number of dialect d as a v2 integer or real literal.
// A whole number is kept only where the source read it exactly and the
// runtime's Integer holds it.
func numberLiteral(text string, d dialect) (translated, *refusal) {
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		return translated{}, &refusal{kind: refusedConstruct, token: text, why: "a hexadecimal literal has no v2 form in the subset"}
	}
	if !strings.ContainsAny(text, ".eE") {
		if len(text) > 1 && text[0] == '0' {
			return translated{}, &refusal{kind: refusedConstruct, token: text, why: "a legacy octal literal"}
		}
		v, err := strconv.ParseInt(text, 10, 64)
		switch {
		case err != nil:
			return translated{}, &refusal{kind: refusedConstruct, token: text,
				why: "the whole number is beyond the " + strconv.FormatInt(math.MaxInt64, 10) + " an Integer holds"}
		case d != dialectJava && v > maxSafeInteger:
			return translated{}, &refusal{kind: refusedConstruct, token: text,
				why: "a script rounds a whole number beyond " + strconv.FormatInt(maxSafeInteger, 10) + " to the nearest floating-point value"}
		}
		return translated{expr: text, scalar: "Integer", atomic: true, lit: "integer"}, nil
	}
	v, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return translated{}, &refusal{kind: refusedSyntax, token: text, why: "the number is not finite"}
	}
	return translated{expr: realLiteral(v), scalar: "Real", atomic: true, lit: "real"}, nil
}

// stringLiteral writes text as a v2 string literal.
func stringLiteral(text string) string { return lexer.StringText(text) }

// numbersAt refuses operands of op that are known not to be numbers.
func numbersAt(op string, sides ...translated) *refusal {
	for _, s := range sides {
		if s.plural {
			return &refusal{kind: refusedType, token: op, why: "an operand is a collection, not a number"}
		}
		if s.held() != "" && !isNumeric(s.scalar) {
			return &refusal{kind: refusedType, token: op, why: "an operand is a " + s.held() + ", not a number"}
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

// arithmeticLoose is the looseness of an arithmetic operator.
func arithmeticLoose(op string) int {
	if op == "+" || op == "-" {
		return looseAdditive
	}
	return looseMultiplicative
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
// ok is false when their known types conflict: a non-scalar with any scalar,
// or two non-scalars with no type in common.
func commonScalar(a, b translated) (string, bool) {
	switch {
	case a.plural != b.plural:
		return "", false
	case len(a.object) > 0 || len(b.object) > 0:
		return "", a.scalar == "" && b.scalar == "" && (len(a.object) == 0 || len(b.object) == 0 || commonObject(a, b) != nil)
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
	case wholeScalar(scalar) && (value.lit == "real" || value.lit == "integer"):
		_, ok := scalarLiteral(value.lit, value.expr, value.expr, scalar)
		return ok
	}
	return false
}

// assignableTo reports whether value may be held by the feature target reads:
// a scalar as assignableScalar tells, a non-scalar by a value of no scalar
// whose type, when known, is the target's or specializes it.
func assignableTo(target, value translated) bool {
	if len(target.object) > 0 || len(value.object) > 0 {
		return target.scalar == "" && value.scalar == "" && conforms(value, target)
	}
	return assignableScalar(target.scalar, value)
}

// conforms reports whether a value of a's non-scalar type is one of b's: the
// same type or one it specializes, or either type unknown.
func conforms(a, b translated) bool {
	if len(a.object) == 0 || len(b.object) == 0 {
		return true
	}
	for _, t := range a.object {
		if t == b.object[0] {
			return true
		}
	}
	return false
}

// commonObject is the most special non-scalar type both a and b are known to
// hold, with what generalizes it; the known one when only one is known.
func commonObject(a, b translated) []string {
	if len(a.object) == 0 {
		return b.object
	}
	for i, t := range a.object {
		if conforms(b, translated{object: []string{t}}) {
			return a.object[i:]
		}
	}
	return nil
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
