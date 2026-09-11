package parser

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// atName reports whether the current token can begin a name segment. A keyword
// of the *other* language is an ordinary name: Xtext reserves a literal only
// inside the grammar declaring it, and the two grammars share only
// KerMLExpressions.xtext (`part chains : T;` is a SysML part named `chains`).
func (p *Parser) atName() bool {
	t := p.peek()
	switch t.Kind {
	case lexer.Identifier, lexer.UnrestrictedName:
		return true
	case lexer.Keyword:
		return !p.reservedWord(t.KeywordID)
	}
	return false
}

// reservedWord reports whether the word is a literal of this file's grammar,
// and so cannot spell a name in it.
func (p *Parser) reservedWord(w string) bool {
	kind := p.src.Kind()
	if kind == source.KindUnknown {
		// A buffer with no model extension is read as SysML, as the
		// nonstandard-notation pass already reads it.
		kind = source.KindSysML
	}
	return lexer.IsKeywordIn(w, kind)
}

// atNameOrKeyword reports whether the current token can begin a name segment,
// including keywords used as identifiers (relaxed parsing for identification).
func (p *Parser) atNameOrKeyword() bool {
	k := p.peek().Kind
	return k == lexer.Identifier || k == lexer.UnrestrictedName || k == lexer.Keyword
}

// parseNameSegment consumes one name token and returns its segment.
func (p *Parser) parseNameSegment() (ast.NameSegment, bool) {
	if !p.atName() {
		return ast.NameSegment{}, false
	}
	tok := p.advance()
	text := p.src.Text(tok.Span)
	// Strip quotes from unrestricted names
	if tok.Kind == lexer.UnrestrictedName && len(text) >= 2 && text[0] == '\'' && text[len(text)-1] == '\'' {
		text = text[1 : len(text)-1]
	}
	return ast.NameSegment{Text: text, Span: tok.Span}, true
}

// parseNameSegmentRelaxed consumes a name token (including keywords) and returns its segment.
// Used in contexts where keywords can serve as identifiers (e.g., declaration names).
func (p *Parser) parseNameSegmentRelaxed() (ast.NameSegment, bool) {
	if !p.atNameOrKeyword() {
		return ast.NameSegment{}, false
	}
	tok := p.advance()
	text := p.src.Text(tok.Span)
	// Strip quotes from unrestricted names
	if tok.Kind == lexer.UnrestrictedName && len(text) >= 2 && text[0] == '\'' && text[len(text)-1] == '\'' {
		text = text[1 : len(text)-1]
	}
	return ast.NameSegment{Text: text, Span: tok.Span}, true
}

// parseQualifiedName parses `[$::] Name (:: Name)*`. It returns nil and
// records a diagnostic if no name is present.
func (p *Parser) parseQualifiedName() *ast.QualifiedName {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()

	global := false
	if p.at(lexer.Dollar) && p.peekN(1).Kind == lexer.ColonColon {
		p.advance() // $
		p.advance() // ::
		global = true
	}

	seg, ok := p.parseNameSegment()
	if !ok {
		if global {
			// `$::` with no following name — still a (degenerate) global name.
			qn := &ast.QualifiedName{Global: true}
			qn.NodeSpan = p.spanFrom(start)
			qn.SetLeadingTrivia(trivia)
			return qn
		}
		p.error(p.peek().Span, "expected a name")
		return nil
	}

	qn := &ast.QualifiedName{Global: global}
	qn.SetSingleton(seg)
	for p.at(lexer.ColonColon) {
		// Do not consume `::` if it introduces `*`/`**` (namespace import wildcard).
		if nk := p.peekN(1).Kind; nk == lexer.Star || nk == lexer.StarStar {
			break
		}
		p.advance() // ::
		next, ok := p.parseNameSegmentRelaxed()
		if !ok {
			p.error(p.peek().Span, "expected a name after '::'")
			break
		}
		qn.Parts = append(qn.Parts, next)
	}

	qn.NodeSpan = p.spanFrom(start)
	qn.SetLeadingTrivia(trivia)
	return qn
}

// parseQualifiedNameRelaxed parses `[$::] Name (:: Name)*` allowing keywords as identifiers.
// Used in contexts where feature chains can start with keywords (e.g., do.startShot).
func (p *Parser) parseQualifiedNameRelaxed() *ast.QualifiedName {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()

	global := false
	if p.at(lexer.Dollar) && p.peekN(1).Kind == lexer.ColonColon {
		p.advance() // $
		p.advance() // ::
		global = true
	}

	seg, ok := p.parseNameSegmentRelaxed()
	if !ok {
		if global {
			// `$::` with no following name — still a (degenerate) global name.
			qn := &ast.QualifiedName{Global: true}
			qn.NodeSpan = p.spanFrom(start)
			qn.SetLeadingTrivia(trivia)
			return qn
		}
		p.error(p.peek().Span, "expected a name")
		return nil
	}

	qn := &ast.QualifiedName{Global: global}
	qn.SetSingleton(seg)
	for p.at(lexer.ColonColon) {
		// Do not consume `::` if it introduces `*`/`**` (namespace import wildcard).
		if nk := p.peekN(1).Kind; nk == lexer.Star || nk == lexer.StarStar {
			break
		}
		p.advance() // ::
		next, ok := p.parseNameSegmentRelaxed()
		if !ok {
			p.error(p.peek().Span, "expected a name after '::'")
			break
		}
		qn.Parts = append(qn.Parts, next)
	}
	qn.NodeSpan = p.spanFrom(start)
	qn.SetLeadingTrivia(trivia)
	return qn
}

// parseIdentification parses `<shortName> name?` or `name` or nothing.
// A missing identification yields a zero-value Identification (no diagnostic).
func (p *Parser) parseIdentification() ast.Identification {
	return p.parseIdentificationStopping()
}

// parseIdentificationStopping parses an identification whose name may not be one of
// stop: those keywords end the declaration rather than naming it.
func (p *Parser) parseIdentificationStopping(stop ...string) ast.Identification {
	var id ast.Identification
	if p.at(lexer.Lt) {
		p.advance() // <
		if seg, ok := p.parseNameSegmentRelaxed(); ok {
			id.ShortName = seg.Text
			id.ShortNameSpan = seg.Span
		} else {
			p.error(p.peek().Span, msgExpectedShortName)
		}
		p.expect(lexer.Gt, msgExpectedCloseAngle)
	}
	// Parse name, but exclude keywords that have special syntax meaning in declaration context
	// (e.g., "default" introduces a value expression, "connect"/"allocate" introduce connector ends, "first"/"do" for succession, "of" for flow payload)
	// A feature specialization keyword states a relationship, not a name, and
	// must read as its symbol does: `<s> references x` is `<s> ::> x`.
	if p.atFeatureSpecialization() {
		return id
	}
	if p.at(lexer.Keyword) {
		kw := p.peek().KeywordID
		for _, s := range stop {
			if kw == s {
				return id
			}
		}
		switch kw {
		case "default", "connect", "allocate", "from", "to", "then", "first", "do", "of":
			// These keywords continue a declaration where the language reserves them.
			if p.reservedWord(kw) {
				return id
			}
		}
		// Any other keyword here is the name the author meant, so it is read as
		// one rather than dropped. Only a word this language reserves needs the
		// quotes of an unrestricted name to spell it (KerML §7.2.4).
		if p.reservedWord(kw) {
			p.warn(p.peek().Span, fmt.Sprintf("%q is a reserved keyword; write '%s' to use it as a name", kw, kw), codeReservedKeywordName)
		}
	}
	if seg, ok := p.parseNameSegmentRelaxed(); ok {
		id.Name = seg.Text
		id.NameSpan = seg.Span
	}
	return id
}

// parseAnnotationIdentification parses the identification of a comment,
// documentation or representation. Its /* */ body ends the identification, so a
// name after that body belongs to the next member: `doc <a> /* ... */ feature q;`.
func (p *Parser) parseAnnotationIdentification() ast.Identification {
	if !p.at(lexer.Lt) {
		return p.parseIdentification()
	}
	var id ast.Identification
	ltOff := p.peek().Span.Offset
	p.advance() // <
	if seg, ok := p.parseNameSegmentRelaxed(); ok {
		id.ShortName = seg.Text
		id.ShortNameSpan = seg.Span
	} else {
		p.error(p.peek().Span, msgExpectedShortName)
	}
	p.expect(lexer.Gt, msgExpectedCloseAngle)
	p.peek() // force any trailing comment into the pending body
	if p.pendingCommentAfter(ltOff) {
		return id
	}
	if seg, ok := p.parseNameSegmentRelaxed(); ok {
		id.Name = seg.Text
		id.NameSpan = seg.Span
	}
	return id
}

// parseRepresentationIdentification parses the identification of a textual
// representation (`<short>` and/or a name), which its `language` keyword ends.
func (p *Parser) parseRepresentationIdentification() ast.Identification {
	var id ast.Identification
	if p.at(lexer.Lt) {
		p.advance() // <
		if seg, ok := p.parseNameSegmentRelaxed(); ok {
			id.ShortName = seg.Text
			id.ShortNameSpan = seg.Span
		} else {
			p.error(p.peek().Span, msgExpectedShortName)
		}
		p.expect(lexer.Gt, msgExpectedCloseAngle)
	}
	if p.atName() && !p.atKeyword("language") {
		if seg, ok := p.parseNameSegmentRelaxed(); ok {
			id.Name = seg.Text
			id.NameSpan = seg.Span
		}
	}
	return id
}

// parseVisibility reads an optional public/private/protected prefix.
func (p *Parser) parseVisibility() ast.Visibility {
	switch {
	case p.acceptKeyword("public"):
		return ast.VisibilityPublic
	case p.acceptKeyword("private"):
		return ast.VisibilityPrivate
	case p.acceptKeyword("protected"):
		return ast.VisibilityProtected
	default:
		return ast.VisibilityDefault
	}
}

// parseMember parses one namespace member: an optional visibility prefix
// followed by a declaration. Import/Alias carry their own visibility and are
// returned directly; other declarations are wrapped in a Membership.
func (p *Parser) parseMember() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()
	vis := p.parseVisibility()

	// Import and Alias hold visibility internally and are not wrapped.
	if p.atKeyword("import") {
		imp := p.parseImport(start, vis)
		imp.SetLeadingTrivia(trivia)
		return imp
	}
	if p.atKeyword("alias") {
		al := p.parseAlias(start, vis)
		al.SetLeadingTrivia(trivia)
		return al
	}
	if p.atRelationshipMember() {
		return p.parseRelationshipMember(start, vis, trivia)
	}
	if en := p.parseMisplacedStateSubaction(start, trivia); en != nil {
		return en
	}

	// A namespace member may be a succession stated without its keyword
	// (SysML v2 8.2.2.13.3): `first a::b then c;`, whose ends are qualified
	// names rather than members of an action's token flow.
	var inner ast.Node
	if p.atKeyword("first") {
		inner = p.parseSuccessionAsUsage(start)
	} else {
		inner = p.parseDeclaration(start)
	}
	if inner == nil {
		// No declaration recognized. Emit an error node spanning the skip.
		en := p.errorNodeSkip(start, "expected a namespace member")
		en.SetLeadingTrivia(trivia)
		return en
	}
	m := &ast.Membership{Visibility: vis, Member: inner}
	m.NodeSpan = p.spanFrom(start)
	m.SetLeadingTrivia(trivia)
	return m
}

// tryParseDeclaration attempts to parse a declaration without committing.
// Returns the parsed node if successful, nil if current position doesn't start a declaration.
// Uses backtracking to avoid consuming tokens on failure.
func (p *Parser) tryParseDeclaration() ast.Node {
	// In a statement position `name = expr;` is an assignment shorthand,
	// not a keyword-less usage; leave it to the caller.
	if p.atName() {
		switch p.peekN(1).Kind {
		case lexer.Eq, lexer.ColonEq:
			return nil
		}
	}

	start := p.peek().Span.Offset
	cp := p.checkpoint()
	defer p.release()

	node := p.parseDeclaration(start)

	// Check if parse succeeded (node returned, not ErrorNode, position advanced)
	if node == nil {
		p.restore(cp)
		return nil
	}

	if _, isError := node.(*ast.ErrorNode); isError {
		p.restore(cp)
		return nil
	}

	// Success: keep the parsed node
	return node
}

// parseDeclaration dispatches on the leading keyword to a declaration parser.
// Returns nil if the current token starts no known (in-scope) declaration.
func (p *Parser) parseDeclaration(start int) ast.Node {
	switch {
	case p.atKeyword("package"), p.atKeyword("library"), p.atKeyword("standard"):
		return p.parsePackage(start)
	case p.atKeyword("namespace"):
		return p.parseNamespace(start)
	case p.atKeyword("dependency"):
		return p.parseDependency(start)
	case p.atKeyword("comment"):
		return p.parseComment(start)
	case p.atKeyword("doc"):
		return p.parseDocumentation(start)
	case p.atTextualRepresentationStart():
		return p.parseTextualRepresentation(start)
	case p.atKeyword("multiplicity"):
		return p.parseMultiplicityDecl(start)
	case p.atKeyword("filter"):
		return p.parseFilter(start)
	case p.atKeyword("locale"):
		// An anonymous comment carrying a locale (SysML.xtext Comment: the
		// `comment` keyword is optional).
		return p.parseAnonymousLocaleComment(start)
	case p.atDefUsageStart():
		return p.parseDefUsage(start)
	case p.atKeywordlessFeature():
		// A feature declared with no keyword: `T1 = 10.0;`, `a : Integer;`,
		// `p5[1] : Real;`, `x;` (KerML.xtext Feature, SysML DefaultReferenceUsage).
		return p.parseDefUsage(start)
	case p.at(lexer.At):
		// A metadata usage is a namespace member as much as it is a body member.
		if pm := p.parseMetadataUsage(start); pm != nil {
			return pm
		}
		return nil
	case p.at(lexer.Hash):
		// Look past `# QualifiedName ...` prefixes for the declaration keyword.
		if p.leadingPrefixIsPackage() {
			return p.parsePackage(start)
		}
		if p.leadingPrefixIsNamespace() {
			return p.parseNamespace(start)
		}
		if p.leadingPrefixIsDependency() {
			return p.parseDependency(start)
		}
		if p.leadingPrefixIsDefUsage() {
			return p.parseDefUsage(start)
		}
		return nil
	default:
		return nil
	}
}

// parseNamespaceBody parses `{ member* }` or `;`. Returns (members, hasBody).
// The caller has already consumed the declaration head up to this point.
func (p *Parser) parseNamespaceBody() ([]ast.Node, bool) {
	if p.accept2(lexer.Semicolon) || !p.expectBodyOrEnd("declaration") {
		return nil, false
	}
	// A package/namespace body has its own notation; it never inherits the
	// enclosing body's (e.g. an interface's default-end allowance).
	defer p.pushBodyContext(bodyOther)()
	var members []ast.Node
	for !p.atEOF() && !p.at(lexer.RBrace) {
		before := p.peek().Span.Offset
		members = append(members, p.parseMember())
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}'")
	return members, true
}

// accept2 is accept that discards the token (convenience for punctuation).
func (p *Parser) accept2(k lexer.Kind) bool {
	_, ok := p.accept(k)
	return ok
}

// declStartKeywords are keywords that begin a namespace-member declaration.
// They serve as recovery sync points so a malformed member does not swallow
// the declaration that follows it.
var declStartKeywords = map[string]bool{
	"package":    true,
	"namespace":  true,
	"library":    true,
	"standard":   true,
	"dependency": true,
	"comment":    true,
	"doc":        true,
	"rep":        true,
	"language":   true,
	"alias":      true,
	"import":     true,
	"public":     true,
	"private":    true,
	"protected":  true,
	"part":       true,
	"attribute":  true,
	"def":        true,
	"abstract":   true,
	"variation":  true,
	"ref":        true,
	"in":         true,
	"out":        true,
	"inout":      true,
	"composite":  true,
	"portion":    true,
	"derived":    true,
	"ordered":    true,
	"nonunique":  true,
	// Remaining def/usage kind keywords (Tier A/B/C).
	"item":         true,
	"occurrence":   true,
	"individual":   true,
	"metadata":     true,
	"enum":         true,
	"view":         true,
	"viewpoint":    true,
	"rendering":    true,
	"concern":      true,
	"connection":   true,
	"flow":         true,
	"port":         true,
	"interface":    true,
	"allocation":   true,
	"binding":      true,
	"action":       true,
	"state":        true,
	"calc":         true,
	"constraint":   true,
	"requirement":  true,
	"case":         true,
	"analysis":     true,
	"verification": true,
	"use":          true,
	"datatype":     true,
	"feature":      true,
	// KerML structural.
	"behavior":  true,
	"assoc":     true,
	"struct":    true,
	"class":     true,
	"predicate": true,
	"bool":      true,
	// Usage-only synonyms.
	"inv":      true, // synonym for constraint (invariant)
	"function": true, // synonym for calc
}

// atMemberSync reports whether the parser sits at a recovery synchronization
// point: EOF, `;`, `}`, `#`, or a declaration-start keyword.
func (p *Parser) atMemberSync() bool {
	if p.atEOF() || p.at(lexer.Semicolon) || p.at(lexer.RBrace) || p.at(lexer.Hash) {
		return true
	}
	t := p.peek()
	return t.Kind == lexer.Keyword && declStartKeywords[t.KeywordID]
}

// errorNodeSkip builds an ErrorNode and skips tokens up to the next member
// sync point (`;`/`}`/`#`/declaration keyword/EOF), leaving that token for the
// enclosing loop. It always advances at least one token (unless already at
// EOF/`;`/`}`) so parsing makes progress.
func (p *Parser) errorNodeSkip(start int, msg string) *ast.ErrorNode {
	p.error(p.peek().Span, msg)
	if !p.atEOF() && !p.at(lexer.Semicolon) && !p.at(lexer.RBrace) {
		p.advance()
	}
	for !p.atMemberSync() {
		p.advance()
	}
	p.accept2(lexer.Semicolon) // consume the terminator if present
	en := &ast.ErrorNode{Message: msg}
	en.NodeSpan = p.spanFrom(start)
	return en
}

// expectCommentBody consumes a trailing /* */ regular comment and returns its
// span. It peeks first to force any trailing comment into the pending feature value.
func (p *Parser) expectCommentBody(start int) source.Span {
	p.peek()
	if sp, ok := p.takePendingComment(); ok {
		return sp
	}
	p.error(p.peek().Span, "expected a /* ... */ comment body")
	return p.spanFrom(start)
}

// pendingCommentAfter reports whether the pending /* */ comment starts at or
// after off, i.e. belongs to the element being parsed rather than an earlier one.
func (p *Parser) pendingCommentAfter(off int) bool {
	return p.hasPendingComment && p.pendingComment.Offset >= off
}

func (p *Parser) parseComment(start int) ast.Node {
	p.advance() // 'comment'
	c := &ast.Comment{}
	// An identification may be a short name alone: `comment <c> /* ... */`. A name
	// after the body belongs to the next member, not to this comment.
	if (p.atName() || p.at(lexer.Lt)) && !p.atKeyword("about") && !p.atKeyword("locale") && !p.pendingCommentAfter(start) {
		c.Ident = p.parseAnnotationIdentification()
	}
	if p.acceptKeyword("about") {
		c.About = p.parseQualifiedNameList()
	}
	if p.acceptKeyword("locale") {
		if tok, ok := p.expect(lexer.String, msgExpectedLocaleString); ok {
			c.Locale = p.src.Text(tok.Span)
		}
	}
	c.BodySpan = p.expectCommentBody(start)
	c.NodeSpan = p.spanFrom(start)
	return c
}

// parseAnonymousLocaleComment parses `locale "en_US" /* ... */`, a Comment
// whose optional `comment` keyword is omitted (SysML.xtext:86).
func (p *Parser) parseAnonymousLocaleComment(start int) ast.Node {
	p.advance() // 'locale'
	c := &ast.Comment{}
	if tok, ok := p.expect(lexer.String, msgExpectedLocaleString); ok {
		c.Locale = p.src.Text(tok.Span)
	}
	c.BodySpan = p.expectCommentBody(start)
	c.NodeSpan = p.spanFrom(start)
	return c
}

func (p *Parser) parseDocumentation(start int) ast.Node {
	p.advance() // 'doc'
	d := &ast.Documentation{}

	// Parse optional identification only if there's no pending comment
	// Pattern: `doc name /* comment */` vs `doc /* comment */` (comment belongs to doc, not name)
	if (p.atName() || p.at(lexer.Lt)) && !p.atKeyword("locale") && !p.pendingCommentAfter(start) {
		d.Ident = p.parseAnnotationIdentification()
	}
	if p.acceptKeyword("locale") {
		if tok, ok := p.expect(lexer.String, msgExpectedLocaleString); ok {
			d.Locale = p.src.Text(tok.Span)
		}
	}
	d.BodySpan = p.expectCommentBody(start)
	d.NodeSpan = p.spanFrom(start)
	return d
}

func (p *Parser) parseTextualRepresentation(start int) ast.Node {
	r := &ast.TextualRepresentation{}
	if p.acceptKeyword("rep") {
		if p.at(lexer.Lt) || (p.atName() && !p.atKeyword("language")) {
			r.Ident = p.parseRepresentationIdentification()
		}
	}
	if !p.acceptKeyword("language") {
		return p.errorNodeSkip(start, "expected 'language'")
	}
	if tok, ok := p.expect(lexer.String, "expected representation language string"); ok {
		r.Language = p.src.Text(tok.Span)
	}
	r.BodySpan = p.expectCommentBody(start)
	r.NodeSpan = p.spanFrom(start)
	return r
}

// parseImport parses `import [all] QualifiedName [::*|::**] body`.
// Visibility has already been consumed by the caller.
func (p *Parser) parseImport(start int, vis ast.Visibility) *ast.Import {
	kw := p.peek()
	p.advance() // 'import' (guaranteed by caller)
	// ImportPrefix makes the indicator mandatory (KerML.xtext ImportPrefix); the
	// bare form still reads unambiguously, so it is a warning the analysis escalates.
	if vis == ast.VisibilityDefault {
		p.warn(kw.Span, msgImportVisibility, codeImportVisibility)
	}
	isAll := p.acceptKeyword("all")

	qn := p.parseQualifiedName()
	imp := &ast.Import{
		Visibility: vis,
		IsAll:      isAll,
		Kind:       ast.ImportMembership,
		Imported:   qn,
	}

	p.parseImportTail(imp)

	imp.Body, imp.HasBody = p.parseNamespaceBody()
	imp.NodeSpan = p.spanFrom(start)
	return imp
}

// parseImportTail parses the wildcard tail and the optional filter expression
// that follow the imported qualified name of an import or an expose, setting
// Kind and IsRecursive on imp accordingly.
//
// Wildcard tail (per KerML.xtext ImportedMembership/ImportedNamespace):
//
//	`:: *`           -> namespace import (may then take `:: **` recursive)
//	`:: **` directly -> recursive MEMBERSHIP import (no `::*`)
//
// Filter expression: `import Package::*[@MetadataType];`
func (p *Parser) parseImportTail(imp *ast.Import) {
	if p.at(lexer.ColonColon) {
		nk := p.peekN(1).Kind
		if nk == lexer.Star {
			p.advance() // ::
			p.advance() // *
			imp.Kind = ast.ImportNamespace
			if p.at(lexer.ColonColon) && p.peekN(1).Kind == lexer.StarStar {
				p.advance() // ::
				p.advance() // **
				imp.IsRecursive = true
			}
		} else if nk == lexer.StarStar {
			p.advance()            // ::
			p.advance()            // **
			imp.IsRecursive = true // recursive membership import; Kind stays ImportMembership
		}
	}

	// A filter package takes one or more filter members (KerML.xtext
	// FilterPackage:200); they select conjunctively, so they combine with `and`.
	for p.at(lexer.LBracket) {
		start := p.peek().Span.Offset
		if imp.FilterExpr != nil {
			start = imp.FilterExpr.Span().Offset
		}
		p.advance() // consume '['
		expr := p.ParseExpression()
		p.expect(lexer.RBracket, "expected ']' after import filter expression")
		if imp.FilterExpr == nil {
			imp.FilterExpr = expr
			continue
		}
		and := &ast.OperatorExpr{
			Operator: ast.OpConditionalAnd,
			Operands: []ast.Node{imp.FilterExpr, expr},
		}
		and.NodeSpan = p.spanFrom(start)
		imp.FilterExpr = and
	}
}

func (p *Parser) parseAlias(start int, vis ast.Visibility) *ast.Alias {
	p.advance() // 'alias'
	id := p.parseIdentification()
	al := &ast.Alias{Visibility: vis, Ident: id}
	if !p.acceptKeyword("for") {
		p.error(p.peek().Span, "expected 'for' in alias")
	} else {
		al.For = p.parseQualifiedName()
	}
	al.Body, al.HasBody = p.parseNamespaceBody()
	al.NodeSpan = p.spanFrom(start)
	return al
}

// parseQualifiedNameList parses `QualifiedName (, QualifiedName)*`.
func (p *Parser) parseQualifiedNameList() []*ast.QualifiedName {
	var list []*ast.QualifiedName
	if qn := p.parseQualifiedName(); qn != nil {
		list = append(list, qn)
	}
	for p.at(lexer.Comma) {
		p.advance() // ,
		if qn := p.parseQualifiedName(); qn != nil {
			list = append(list, qn)
		}
	}
	return list
}

// parseDependency parses
// `[# prefixes] dependency [<id> from] clients to suppliers body`.
func (p *Parser) parseDependency(start int) ast.Node {
	prefixes := p.parsePrefixMetadata()
	if !p.acceptKeyword("dependency") {
		return p.errorNodeSkip(start, "expected 'dependency'")
	}
	dep := &ast.Dependency{Prefixes: prefixes}

	// Optional `<id> [name] from`. The `from` keyword disambiguates: an
	// identification is present only if a `from` follows it.
	if p.identificationThenFrom() {
		dep.Ident = p.parseIdentification()
		p.acceptKeyword("from") // guaranteed
	}

	dep.Clients = p.parseQualifiedNameList()
	if !p.acceptKeyword("to") {
		p.error(p.peek().Span, "expected 'to' in dependency")
	} else {
		dep.Suppliers = p.parseQualifiedNameList()
	}
	dep.Body, dep.HasBody = p.parseNamespaceBody()
	dep.NodeSpan = p.spanFrom(start)
	return dep
}

// identificationThenFrom reports whether the upcoming tokens form an
// identification (`<x> y` / `y`) immediately followed by `from`.
func (p *Parser) identificationThenFrom() bool {
	i := 0
	if p.peekN(i).Kind == lexer.Lt {
		i++ // <
		if k := p.peekN(i).Kind; k == lexer.Identifier || k == lexer.UnrestrictedName {
			i++
		}
		if p.peekN(i).Kind == lexer.Gt {
			i++
		}
	}
	if k := p.peekN(i).Kind; k == lexer.Identifier || k == lexer.UnrestrictedName {
		i++
	}
	t := p.peekN(i)
	return t.Kind == lexer.Keyword && t.KeywordID == "from"
}

// parsePrefixMetadata parses zero or more `# QualifiedName` prefix annotations
// (SysML.xtext PrefixMetadataUsage, KerML.xtext PrefixMetadataFeature).
func (p *Parser) parsePrefixMetadata() []*ast.PrefixMetadata {
	var prefixes []*ast.PrefixMetadata
	for p.at(lexer.Hash) {
		hash := p.advance()
		start := hash.Span.Offset
		// PrefixMetadataUsage names a metaclass by QualifiedName (KerML.xtext:1014,
		// SysML.xtext:181): a keyword after `#` starts the declaration, not the name.
		if !p.atName() && !(p.at(lexer.Dollar) && p.peekN(1).Kind == lexer.ColonColon) {
			msg := "expected a metadata feature name after '#'"
			if t := p.peek(); t.Kind == lexer.Keyword {
				msg = fmt.Sprintf("expected a metadata feature name after '#': '%s' is a keyword", t.KeywordID)
			}
			p.error(hash.Span, msg)
			continue
		}
		qn := p.parseQualifiedName()
		if qn == nil {
			continue
		}
		pm := &ast.PrefixMetadata{Type: qn}
		pm.NodeSpan = p.spanFrom(start)
		prefixes = append(prefixes, pm)
	}
	return prefixes
}

// parsePackage parses `[standard] [library] package <id> body`.
// Prefix metadata may precede `package`; it is consumed here.
func (p *Parser) parsePackage(start int) ast.Node {
	prefixes := p.parsePrefixMetadata()
	isStandard := p.acceptKeyword("standard")
	isLibrary := p.acceptKeyword("library")
	if !p.acceptKeyword("package") {
		return p.errorNodeSkip(start, "expected 'package'")
	}
	id := p.parseIdentification()
	members, hasBody := p.parseNamespaceBody()
	pkg := &ast.Package{
		Prefixes:   prefixes,
		Ident:      id,
		IsLibrary:  isLibrary,
		IsStandard: isStandard,
		Members:    members,
		HasBody:    hasBody,
	}
	pkg.NodeSpan = p.spanFrom(start)
	return pkg
}

// parseNamespace parses `namespace <id> body`.
func (p *Parser) parseNamespace(start int) ast.Node {
	prefixes := p.parsePrefixMetadata()
	if !p.acceptKeyword("namespace") {
		return p.errorNodeSkip(start, "expected 'namespace'")
	}
	id := p.parseIdentification()
	members, hasBody := p.parseNamespaceBody()
	ns := &ast.Namespace{Prefixes: prefixes, Ident: id, Members: members, HasBody: hasBody}
	ns.NodeSpan = p.spanFrom(start)
	return ns
}

// prefixLookahead returns the buffer index of the token following all
// leading `# [$::] QualifiedName` prefixes, without consuming anything.
func (p *Parser) prefixLookahead() int {
	return p.prefixMetadataEndAt(0)
}

func (p *Parser) leadingPrefixIsPackage() bool {
	t := p.peekN(p.prefixLookahead())
	return t.Kind == lexer.Keyword && (t.KeywordID == "package" || t.KeywordID == "library" || t.KeywordID == "standard")
}

func (p *Parser) leadingPrefixIsNamespace() bool {
	t := p.peekN(p.prefixLookahead())
	return t.Kind == lexer.Keyword && t.KeywordID == "namespace"
}

// A dependency takes prefix metadata like any element (SysML.xtext:55-57).
func (p *Parser) leadingPrefixIsDependency() bool {
	t := p.peekN(p.prefixLookahead())
	return t.Kind == lexer.Keyword && t.KeywordID == "dependency"
}

func (p *Parser) leadingPrefixIsDefUsage() bool {
	i := p.prefixLookahead() // skip past all #QualifiedName prefixes
	t := p.peekN(i)
	// SysML v2 §7.27.4: a user keyword may declare a usage on its own, with no
	// language-defined keyword (`#failure 'device shutoff';`).
	if t.Kind == lexer.Identifier || t.Kind == lexer.UnrestrictedName {
		return true
	}
	if t.Kind != lexer.Keyword {
		return false
	}
	// Check if keyword is def/usage keyword OR 'def' modifier
	if t.KeywordID == "def" {
		return true // explicit 'def' after prefixes
	}
	// `#M connect a to b;` — `connect` begins an anonymous connection usage
	// without being a kind keyword (SysML.xtext ConnectionUsage).
	if t.KeywordID == "connect" {
		return true
	}
	// `use case` is one kind spelled in two words: `#structuredUseCase use case u;`
	if t.KeywordID == "use" {
		n := p.peekN(i + 1)
		return n.Kind == lexer.Keyword && n.KeywordID == "case"
	}
	_, isDef := p.definitionKind(t.KeywordID)
	_, isUsage := p.usageKind(t.KeywordID)
	return isDef || isUsage
}

// parseFilter parses `filter OwnedExpression ;` (ElementFilterMember).
func (p *Parser) parseFilter(start int) ast.Node {
	p.advance() // filter
	expr := p.ParseExpression()
	p.expectSemicolon("filter expression")
	f := &ast.FilterMember{Condition: expr}
	f.NodeSpan = p.spanFrom(start)
	return f
}

// parseMultiplicityDecl parses the two Multiplicity forms of KerML.xtext:754 —
// `multiplicity <id> [range] ;|{ members }` (a MultiplicityRange, e.g.
// exactlyOne [1..1]) and `multiplicity <id> subsets f ;|{ members }` (a
// MultiplicitySubset, which states its bounds by subsetting another
// multiplicity instead of writing them).
func (p *Parser) parseMultiplicityDecl(start int) ast.Node {
	p.advance() // multiplicity

	// Parse identification (name)
	ident := p.parseIdentification()

	var mult *ast.Multiplicity
	var subsets *ast.QualifiedName
	switch {
	case p.at(lexer.LBracket):
		mult = p.parseMultiplicity()
	case p.at(lexer.ColonGt) || p.atKeyword("subsets"):
		p.advance() // ':>' or 'subsets'
		subsets = p.parseQualifiedName()
	}

	// Parse body or semicolon
	members, hasBody := p.parseNamespaceBody()

	md := &ast.MultiplicityDecl{
		Ident:   ident,
		Range:   mult,
		Subsets: subsets,
		Members: members,
		HasBody: hasBody,
	}
	md.NodeSpan = p.spanFrom(start)
	return md
}
