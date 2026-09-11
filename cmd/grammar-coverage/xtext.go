package main

import (
	"fmt"
	"sort"
	"strings"
)

// Kind is how a production is declared in an Xtext grammar.
type Kind string

const (
	// KindRule is an ordinary parser rule.
	KindRule Kind = "rule"
	// KindFragment is a `fragment` rule: inlined into its callers, so it has no
	// object of its own.
	KindFragment Kind = "fragment"
	// KindEnum is an `enum` rule mapping literals onto enumeration values.
	KindEnum Kind = "enum"
	// KindTerminal is a `terminal` rule: matched by the lexer over characters
	// rather than by keywords.
	KindTerminal Kind = "terminal"
)

// Production is one rule, fragment, enum or terminal of a grammar.
type Production struct {
	// Grammar is the grammar file's base name, e.g. "KerML.xtext".
	Grammar string
	Name    string
	Kind    Kind
	// Line is the 1-based line the declaration starts on.
	Line int
	// Returns is the metamodel type the rule constructs, empty when the rule
	// declares none.
	Returns string
	// Override marks a rule carrying Xtext's `@Override` annotation, i.e. one
	// replacing a rule of the inherited grammar.
	Override bool
	// Body is the parsed right-hand side. It is nil for terminals, whose bodies
	// describe character sets rather than notation.
	Body expr
}

// Literals returns every terminal literal the production's own body uses,
// sorted and deduplicated. Literals reached only through a called rule belong
// to that rule, not to this one.
func (p Production) Literals() []string {
	seen := map[string]bool{}
	collectLiterals(p.Body, seen)
	out := make([]string, 0, len(seen))
	for lit := range seen {
		out = append(out, lit)
	}
	sort.Strings(out)
	return out
}

// expr is a parsed Xtext right-hand side. Elements that consume no input of
// their own — actions, cross-references, rule calls — collapse to refExpr,
// because this tool only reasons about literals a production spells out itself.
type expr interface{ isExpr() }

type litExpr struct{ Value string }
type refExpr struct{ Name string }
type seqExpr struct{ Items []expr }
type altExpr struct{ Items []expr }

// optExpr is a `?` or `*` cardinality: input can exercise the production
// without matching it at all.
type optExpr struct{ Item expr }

func (litExpr) isExpr() { /* marker: closed expr set */ }
func (refExpr) isExpr() { /* marker: closed expr set */ }
func (seqExpr) isExpr() { /* marker: closed expr set */ }
func (altExpr) isExpr() { /* marker: closed expr set */ }
func (optExpr) isExpr() { /* marker: closed expr set */ }

func collectLiterals(e expr, out map[string]bool) {
	switch v := e.(type) {
	case litExpr:
		out[v.Value] = true
	case seqExpr:
		for _, item := range v.Items {
			collectLiterals(item, out)
		}
	case altExpr:
		for _, item := range v.Items {
			collectLiterals(item, out)
		}
	case optExpr:
		collectLiterals(v.Item, out)
	}
}

// Grammar is one parsed .xtext file.
type Grammar struct {
	// Name is the file's base name, e.g. "KerML.xtext".
	Name string
	// Declared is the qualified name in the grammar declaration.
	Declared string
	// Extends is the qualified name of the grammar this one is declared `with`,
	// whose rules it inherits and may override. Empty when there is none.
	Extends     string
	Productions []Production
}

// ParseGrammar extracts the productions of one Xtext grammar. The name is
// recorded on every production and used in errors.
func ParseGrammar(name, src string) (*Grammar, error) {
	toks, err := scanXtext(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	p := &grammarParser{out: Grammar{Name: name}, toks: toks}
	if err := p.parseGrammar(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &p.out, nil
}

type grammarParser struct {
	out  Grammar
	toks []token
	pos  int
}

func (p *grammarParser) peek() token {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return token{kind: tokEOF}
}

func (p *grammarParser) next() token {
	t := p.peek()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return t
}

func (p *grammarParser) at(kind tokenKind, text string) bool {
	t := p.peek()
	return t.kind == kind && t.text == text
}

func (p *grammarParser) accept(kind tokenKind, text string) bool {
	if p.at(kind, text) {
		p.pos++
		return true
	}
	return false
}

// expect consumes the given token or reports what stood in its place.
func (p *grammarParser) expect(kind tokenKind, text string) error {
	if !p.at(kind, text) {
		t := p.peek()
		return fmt.Errorf("line %d: expected %q, found %q", t.line, text, t.text)
	}
	p.next()
	return nil
}

// keyword reports whether the current token is the given declaration keyword.
// Rule names are capitalised throughout these grammars, so a lower-case word
// followed by an identifier is a keyword rather than a rule being declared.
func (p *grammarParser) keyword(word string) bool {
	if !p.at(tokIdent, word) {
		return false
	}
	return p.pos+1 < len(p.toks) && p.toks[p.pos+1].kind == tokIdent
}

func (p *grammarParser) parseGrammar() error {
	override := false
	for p.peek().kind != tokEOF {
		switch {
		case p.at(tokPunct, "@"):
			p.next()
			name := p.next()
			if name.kind != tokIdent {
				return fmt.Errorf("line %d: expected an annotation name, found %q", name.line, name.text)
			}
			if name.text == "Override" {
				override = true
			}
		case p.keyword("grammar"):
			p.next()
			p.out.Declared = p.skipQualifiedName()
			if p.accept(tokIdent, "with") {
				p.out.Extends = p.skipQualifiedName()
			}
			if p.accept(tokIdent, "hidden") {
				if err := p.skipBalanced("(", ")"); err != nil {
					return err
				}
			}
		case p.at(tokIdent, "import") && p.pos+1 < len(p.toks) && p.toks[p.pos+1].kind == tokString:
			p.next()
			if t := p.next(); t.kind != tokString {
				return fmt.Errorf("line %d: expected an import URI, found %q", t.line, t.text)
			}
			if p.accept(tokIdent, "as") {
				p.next()
			}
		default:
			prod, err := p.parseProduction(override)
			if err != nil {
				return err
			}
			prod.Grammar = p.out.Name
			p.out.Productions = append(p.out.Productions, prod)
			override = false
		}
	}
	return nil
}

func (p *grammarParser) parseProduction(override bool) (Production, error) {
	prod := Production{Kind: KindRule, Override: override}
	switch {
	case p.keyword("terminal"):
		p.next()
		// `terminal fragment X` is still lexer-level.
		p.accept(tokIdent, "fragment")
		prod.Kind = KindTerminal
	case p.keyword("fragment"):
		p.next()
		prod.Kind = KindFragment
	case p.keyword("enum"):
		p.next()
		prod.Kind = KindEnum
	}

	name := p.next()
	if name.kind != tokIdent {
		return prod, fmt.Errorf("line %d: expected a production name, found %q", name.line, name.text)
	}
	prod.Name = name.text
	prod.Line = name.line

	if p.accept(tokIdent, "returns") {
		prod.Returns = p.skipQualifiedName()
	}
	if err := p.expect(tokPunct, ":"); err != nil {
		return prod, fmt.Errorf("production %s: %w", prod.Name, err)
	}

	if prod.Kind == KindTerminal {
		// Terminal bodies are character sets and ranges; the literals in them
		// are not notation, so they are skipped rather than mis-reported.
		if err := p.skipToSemicolon(); err != nil {
			return prod, fmt.Errorf("terminal %s: %w", prod.Name, err)
		}
		return prod, nil
	}

	body, err := p.parseAlternation()
	if err != nil {
		return prod, fmt.Errorf("production %s: %w", prod.Name, err)
	}
	prod.Body = body
	if err := p.expect(tokPunct, ";"); err != nil {
		return prod, fmt.Errorf("production %s: %w", prod.Name, err)
	}
	return prod, nil
}

func (p *grammarParser) skipQualifiedName() string {
	var parts []string
	for {
		t := p.peek()
		if t.kind != tokIdent {
			break
		}
		parts = append(parts, p.next().text)
		switch {
		case p.accept(tokPunct, "::"):
			parts = append(parts, "::")
		case p.accept(tokPunct, "."):
			parts = append(parts, ".")
		default:
			return strings.Join(parts, "")
		}
	}
	return strings.Join(parts, "")
}

func (p *grammarParser) skipBalanced(open, closer string) error {
	if err := p.expect(tokPunct, open); err != nil {
		return err
	}
	depth := 1
	for depth > 0 {
		t := p.next()
		switch {
		case t.kind == tokEOF:
			return fmt.Errorf("unterminated %q", open)
		case t.kind == tokPunct && t.text == open:
			depth++
		case t.kind == tokPunct && t.text == closer:
			depth--
		}
	}
	return nil
}

func (p *grammarParser) skipToSemicolon() error {
	for {
		t := p.next()
		switch {
		case t.kind == tokEOF:
			return fmt.Errorf("unterminated body")
		case t.kind == tokPunct && t.text == ";":
			return nil
		}
	}
}

func (p *grammarParser) parseAlternation() (expr, error) {
	first, err := p.parseSequence()
	if err != nil {
		return nil, err
	}
	items := []expr{first}
	for p.accept(tokPunct, "|") {
		next, err := p.parseSequence()
		if err != nil {
			return nil, err
		}
		items = append(items, next)
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return altExpr{Items: items}, nil
}

func (p *grammarParser) parseSequence() (expr, error) {
	var items []expr
	for {
		t := p.peek()
		if t.kind == tokEOF || (t.kind == tokPunct && (t.text == "|" || t.text == ")" || t.text == ";")) {
			break
		}
		item, err := p.parseElement()
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, item)
		}
	}
	switch len(items) {
	case 0:
		return refExpr{Name: "empty"}, nil
	case 1:
		return items[0], nil
	}
	return seqExpr{Items: items}, nil
}

func (p *grammarParser) parseElement() (expr, error) {
	item, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.accept(tokPunct, "?"), p.accept(tokPunct, "*"):
			item = optExpr{Item: item}
		case p.accept(tokPunct, "+"):
			// One or more: the element itself is still required.
		default:
			return item, nil
		}
	}
}

func (p *grammarParser) parsePrimary() (expr, error) {
	t := p.peek()
	switch {
	case t.kind == tokLiteral:
		p.next()
		return litExpr{Value: t.text}, nil
	case t.kind == tokPunct && (t.text == "->" || t.text == "=>"):
		// Syntactic predicate: a hint to the parser generator, not input.
		p.next()
		return p.parsePrimary()
	case t.kind == tokPunct && t.text == "(":
		p.next()
		inner, err := p.parseAlternation()
		if err != nil {
			return nil, err
		}
		if err := p.expect(tokPunct, ")"); err != nil {
			return nil, err
		}
		return inner, nil
	case t.kind == tokPunct && t.text == "{":
		// An action assigns a type; it consumes nothing.
		if err := p.skipBalanced("{", "}"); err != nil {
			return nil, err
		}
		return nil, nil
	case t.kind == tokPunct && t.text == "[":
		// A cross-reference consumes a name, never a literal.
		if err := p.skipBalanced("[", "]"); err != nil {
			return nil, err
		}
		return refExpr{Name: "crossReference"}, nil
	case t.kind == tokIdent:
		name := p.skipQualifiedName()
		if p.accept(tokPunct, "=") || p.accept(tokPunct, "+=") || p.accept(tokPunct, "?=") {
			return p.parsePrimary()
		}
		return refExpr{Name: name}, nil
	}
	return nil, fmt.Errorf("line %d: unexpected %q in a production body", t.line, t.text)
}
