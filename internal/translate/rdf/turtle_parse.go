package rdf

import (
	"fmt"
	"strings"
	"unicode"
)

// ParseError reports the line at which a Turtle document could not be read.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string { return fmt.Sprintf("turtle: line %d: %s", e.Line, e.Msg) }

// ParseTurtle reads the subset of Turtle that WriteTurtle emits: @prefix and
// @base directives, IRI and prefixed-name terms, quoted literals with optional
// datatype or language tag, the `a` keyword, and the ';' and ',' groupings.
//
// Blank nodes ('[]', '_:x'), collections ('( ... )') and the numeric and
// boolean literal shorthands are rejected rather than approximated: this reader
// exists to convert a graph back into source, and a construct it cannot
// represent has to surface as an error instead of a silently smaller model.
func ParseTurtle(data []byte) (*Graph, error) {
	p := &parser{src: string(data), line: 1, prefixes: map[string]string{}}
	g := &Graph{seen: map[Triple]bool{}, Prefixes: map[string]string{}}
	for {
		p.skipIgnorable()
		if p.eof() {
			break
		}
		if p.peek() == '@' || p.hasKeyword("PREFIX") || p.hasKeyword("BASE") {
			if err := p.directive(); err != nil {
				return nil, err
			}
			continue
		}
		if err := p.statement(g); err != nil {
			return nil, err
		}
	}
	for label, ns := range p.prefixes {
		g.Prefixes[label] = ns
	}
	return g, nil
}

type parser struct {
	src      string
	pos      int
	line     int
	base     string
	prefixes map[string]string
}

func (p *parser) errf(format string, args ...any) error {
	return &ParseError{Line: p.line, Msg: fmt.Sprintf(format, args...)}
}

func (p *parser) eof() bool { return p.pos >= len(p.src) }

func (p *parser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.src[p.pos]
}

func (p *parser) next() byte {
	c := p.src[p.pos]
	p.pos++
	if c == '\n' {
		p.line++
	}
	return c
}

// skipIgnorable consumes whitespace and comments.
func (p *parser) skipIgnorable() {
	for !p.eof() {
		switch c := p.peek(); {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			p.next()
		case c == '#':
			for !p.eof() && p.peek() != '\n' {
				p.next()
			}
		default:
			return
		}
	}
}

// hasKeyword reports whether the input continues with a bare keyword. The
// keyword form is always followed by whitespace, so a longer word or a prefixed
// name that starts with the same letters (`base:Thing`) is not one.
func (p *parser) hasKeyword(word string) bool {
	end := p.pos + len(word)
	if len(p.src) < end || !strings.EqualFold(p.src[p.pos:end], word) {
		return false
	}
	if end == len(p.src) {
		return false
	}
	switch p.src[end] {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

// directive reads '@prefix'/'PREFIX' and '@base'/'BASE'.
func (p *parser) directive() error {
	terminated := false
	if p.peek() == '@' {
		p.next()
		terminated = true
	}
	word := p.readWhile(func(r rune) bool { return unicode.IsLetter(r) })
	switch strings.ToLower(word) {
	case "prefix":
		p.skipIgnorable()
		label := p.readWhile(func(r rune) bool { return r != ':' })
		if p.eof() || p.peek() != ':' {
			return p.errf("expected ':' after prefix label")
		}
		p.next()
		p.skipIgnorable()
		iri, err := p.readIRIRef()
		if err != nil {
			return err
		}
		p.prefixes[strings.TrimSpace(label)] = iri
	case "base":
		p.skipIgnorable()
		iri, err := p.readIRIRef()
		if err != nil {
			return err
		}
		p.base = iri
	default:
		return p.errf("unknown directive %q", word)
	}
	p.skipIgnorable()
	if terminated {
		if p.eof() || p.peek() != '.' {
			return p.errf("expected '.' to end directive")
		}
		p.next()
	} else if !p.eof() && p.peek() == '.' {
		p.next()
	}
	return nil
}

// statement reads one subject with its predicate-object list.
func (p *parser) statement(g *Graph) error {
	subject, err := p.readTerm()
	if err != nil {
		return err
	}
	if !subject.IsIRI() {
		return p.errf("subject must be an IRI")
	}
	for {
		p.skipIgnorable()
		predicate, err := p.readPredicate()
		if err != nil {
			return err
		}
		for {
			p.skipIgnorable()
			object, err := p.readTerm()
			if err != nil {
				return err
			}
			g.Add(subject, predicate, object)
			p.skipIgnorable()
			if !p.eof() && p.peek() == ',' {
				p.next()
				continue
			}
			break
		}
		p.skipIgnorable()
		if p.eof() {
			return p.errf("expected '.' to end statement")
		}
		switch p.peek() {
		case ';':
			p.next()
			p.skipIgnorable()
			// A trailing ';' before '.' is legal Turtle.
			if !p.eof() && p.peek() == '.' {
				p.next()
				return nil
			}
			continue
		case '.':
			p.next()
			return nil
		default:
			return p.errf("expected ';' or '.', found %q", string(p.peek()))
		}
	}
}

func (p *parser) readPredicate() (Term, error) {
	if p.hasKeyword("a") {
		// 'a' is only the rdf:type keyword when a term does not follow it.
		if p.pos+1 >= len(p.src) || isTermBreak(p.src[p.pos+1]) {
			p.next()
			return IRI(RDFType), nil
		}
	}
	term, err := p.readTerm()
	if err != nil {
		return Term{}, err
	}
	if !term.IsIRI() {
		return Term{}, p.errf("predicate must be an IRI")
	}
	return term, nil
}

func isTermBreak(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '#'
}

func (p *parser) readTerm() (Term, error) {
	p.skipIgnorable()
	if p.eof() {
		return Term{}, p.errf("unexpected end of document")
	}
	switch c := p.peek(); c {
	case '<':
		iri, err := p.readIRIRef()
		if err != nil {
			return Term{}, err
		}
		return IRI(iri), nil
	case '"', '\'':
		return p.readLiteral()
	case '[', ']':
		return Term{}, p.errf("blank nodes are not supported")
	case '(', ')':
		return Term{}, p.errf("RDF collections are not supported")
	case '_':
		return Term{}, p.errf("blank node labels are not supported")
	}
	return p.readPrefixedName()
}

func (p *parser) readIRIRef() (string, error) {
	if p.eof() || p.peek() != '<' {
		return "", p.errf("expected '<' to start an IRI")
	}
	p.next()
	var b strings.Builder
	for {
		if p.eof() {
			return "", p.errf("unterminated IRI")
		}
		c := p.next()
		if c == '>' {
			break
		}
		if c == '\n' {
			return "", p.errf("unterminated IRI")
		}
		if c == '\\' && !p.eof() && (p.peek() == 'u' || p.peek() == 'U') {
			r, err := p.readUnicodeEscape(p.next())
			if err != nil {
				return "", err
			}
			b.WriteRune(r)
			continue
		}
		b.WriteByte(c)
	}
	iri := b.String()
	if p.base != "" && !strings.Contains(iri, ":") {
		iri = resolveIRI(p.base, iri)
	}
	return iri, nil
}

func (p *parser) readPrefixedName() (Term, error) {
	start := p.pos
	label := p.readWhile(func(r rune) bool {
		return r != ':' && !unicode.IsSpace(r) && r != ';' && r != ',' && r != '.'
	})
	if p.eof() || p.peek() != ':' {
		p.pos = start
		return Term{}, p.errf("unrecognized term")
	}
	p.next()
	local := p.readWhile(func(r rune) bool {
		return !unicode.IsSpace(r) && r != ';' && r != ',' && !(r == '.' && p.atStatementEnd())
	})
	local = strings.TrimSuffix(local, ".")
	ns, ok := p.prefixes[label]
	if !ok {
		return Term{}, p.errf("undefined prefix %q", label)
	}
	return IRI(ns + local), nil
}

// atStatementEnd reports whether the '.' at the cursor terminates a statement
// rather than being part of a prefixed local name (`elmt:Demo.Vehicle`).
func (p *parser) atStatementEnd() bool {
	for i := p.pos + 1; i < len(p.src); i++ {
		switch p.src[i] {
		case ' ', '\t', '\r':
			continue
		case '\n', '#':
			return true
		default:
			return false
		}
	}
	return true
}

func (p *parser) readLiteral() (Term, error) {
	quote := p.peek()
	long := false
	if strings.HasPrefix(p.src[p.pos:], strings.Repeat(string(quote), 3)) {
		long = true
		p.next()
		p.next()
		p.next()
	} else {
		p.next()
	}
	var b strings.Builder
	for {
		if p.eof() {
			return Term{}, p.errf("unterminated string literal")
		}
		c := p.peek()
		if c == '\\' {
			p.next()
			if p.eof() {
				return Term{}, p.errf("unterminated escape")
			}
			esc := p.next()
			switch esc {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case '\\', '"', '\'':
				b.WriteByte(esc)
			case 'u', 'U':
				value, err := p.readUnicodeEscape(esc)
				if err != nil {
					return Term{}, err
				}
				b.WriteRune(value)
			default:
				return Term{}, p.errf("unknown escape \\%c", esc)
			}
			continue
		}
		if c == quote {
			if !long {
				p.next()
				break
			}
			if strings.HasPrefix(p.src[p.pos:], strings.Repeat(string(quote), 3)) {
				p.next()
				p.next()
				p.next()
				break
			}
		}
		if c == '\n' && !long {
			return Term{}, p.errf("newline in short string literal")
		}
		b.WriteByte(p.next())
	}
	if !p.eof() && p.peek() == '@' {
		p.next()
		lang := p.readWhile(func(r rune) bool {
			return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-'
		})
		return LangString(b.String(), lang), nil
	}
	if strings.HasPrefix(p.src[p.pos:], "^^") {
		p.next()
		p.next()
		datatype, err := p.readTerm()
		if err != nil {
			return Term{}, err
		}
		if !datatype.IsIRI() {
			return Term{}, p.errf("datatype must be an IRI")
		}
		return TypedLiteral(b.String(), datatype.Value), nil
	}
	return String(b.String()), nil
}

// readUnicodeEscape reads the hex digits of a \u (four) or \U (eight) escape
// whose letter esc was just consumed.
func (p *parser) readUnicodeEscape(esc byte) (rune, error) {
	width := 4
	if esc == 'U' {
		width = 8
	}
	if len(p.src)-p.pos < width {
		return 0, p.errf("truncated unicode escape")
	}
	var value rune
	for i := 0; i < width; i++ {
		digit, ok := hexDigit(p.next())
		if !ok {
			return 0, p.errf("invalid unicode escape")
		}
		value = value<<4 | rune(digit)
	}
	return value, nil
}

func hexDigit(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func (p *parser) readWhile(keep func(rune) bool) string {
	start := p.pos
	for !p.eof() {
		r := rune(p.peek())
		if !keep(r) {
			break
		}
		p.next()
	}
	return p.src[start:p.pos]
}

// resolveIRI resolves a relative reference against base as RFC 3986 section 5.2
// does, on the text as written; a base with no hierarchy has it appended.
func resolveIRI(base, ref string) string {
	scheme := strings.Index(base, ":")
	if scheme < 0 || !strings.HasPrefix(base[scheme+1:], "/") {
		return base + ref
	}
	base, _, _ = strings.Cut(base, "#")
	stem, baseQuery, _ := strings.Cut(base, "?")
	root := scheme + 1
	if strings.HasPrefix(stem[root:], "//") {
		if slash := strings.Index(stem[root+2:], "/"); slash >= 0 {
			root += 2 + slash
		} else {
			root = len(stem)
		}
	}
	rest, fragment, hasFragment := strings.Cut(ref, "#")
	path, query, hasQuery := strings.Cut(rest, "?")
	out := stem[:root]
	switch {
	case path == "":
		out = stem
		if !hasQuery && baseQuery != "" {
			query, hasQuery = baseQuery, true
		}
	case strings.HasPrefix(path, "/"):
		out += removeDotSegments(path)
	case root == len(stem):
		out += removeDotSegments("/" + path)
	default:
		out += removeDotSegments(stem[root:strings.LastIndex(stem, "/")+1] + path)
	}
	if hasQuery {
		out += "?" + query
	}
	if hasFragment {
		out += "#" + fragment
	}
	return out
}

// removeDotSegments drops the "." and ".." segments of a path (RFC 3986 section 5.2.4).
func removeDotSegments(path string) string {
	var kept []string
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		last := i == len(segments)-1
		switch segment {
		case ".":
			if last {
				kept = append(kept, "")
			}
		case "..":
			if len(kept) > 1 {
				kept = kept[:len(kept)-1]
			}
			if last {
				kept = append(kept, "")
			}
		default:
			kept = append(kept, segment)
		}
	}
	if len(kept) == 1 && kept[0] == "" && strings.HasPrefix(path, "/") {
		return "/"
	}
	return strings.Join(kept, "/")
}
