package parser

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Parser is a hand-written recursive-descent parser over a lexer token
// stream. It buffers non-trivia tokens for lookahead and collects
// diagnostics; it always produces a tree (ErrorNodes for bad input).
type Parser struct {
	src *source.SourceFile
	lx  *lexer.Lexer
	// buf is a window of the non-trivia token stream: buf[i] is stream token
	// base+i, and pos is the cursor's stream index.
	buf  []lexer.Token
	base int
	pos  int
	// checkpoints counts the outstanding checkpoints; they pin the window.
	checkpoints int
	triv        []ast.Trivia // trivia pending attachment to the next node
	// Diagnostics are syntax errors: input the parser could not read as
	// well-formed SysML.
	Diagnostics []Diagnostic
	// Warnings are findings on input that parsed into the tree the author
	// intended but is not well-formed SysML, such as a reserved keyword written
	// as a declaration name.
	Warnings []Diagnostic

	// calcBodyDepth counts the calculation bodies being parsed, so a `return`
	// reached in a statement position inside one is read as the result
	// parameter it declares rather than as an unknown action keyword.
	calcBodyDepth int

	pendingComment    source.Span // span of the most recent /* */ regular comment
	hasPendingComment bool

	// constraintCalcDepth counts the calculation bodies that are constraint
	// bodies, whose bare expressions are the conditions the constraint states.
	constraintCalcDepth int

	// inEnumBody is set while an enumeration body's direct members are parsed,
	// where a bare name is an enumerated value rather than a default reference usage.
	inEnumBody bool

	// effectDepth counts the transition effects being parsed, whose statement is
	// closed by the transition's next clause rather than by ';'.
	effectDepth int

	// effectStmtStart is the source offset of the statement written as the
	// innermost transition effect, which the transition's own ';' terminates
	// (SysML.xtext EffectBehaviorUsage carries no ';'). It is only meaningful
	// while effectDepth > 0, and distinguishes that statement from the ones
	// nested inside its body, which end with their own ';'.
	effectStmtStart int

	// bodyCtx is the stack of enclosing body notations; only the innermost
	// matters (see bodyContext).
	bodyCtx []bodyContext
}

// bodyContext is the notation of the body being parsed, for members whose grammar
// depends on it: interface default ends (SysML v2 8.2.2.14) and the members only
// one body kind offers (see bodyAdmitsMember).
type bodyContext int

const (
	bodyOther bodyContext = iota
	bodyInterface
	bodyAction
	bodyState
	bodyCalc
	bodyCase
	// bodyRequirement is a RequirementBody: requirement, concern and viewpoint
	// bodies, and those of the usages written with them (satisfy, frame, objective).
	bodyRequirement
	// bodyViewDef is a ViewDefinitionBodyItem body, bodyView a ViewBodyItem one:
	// only the view usage admits `expose`.
	bodyViewDef
	bodyView
)

// carriesActions reports whether the body is action-carrying (action, state,
// calc, case), whose `first` opens an InitialNodeMember rather than a SuccessionAsUsage.
func (c bodyContext) carriesActions() bool {
	switch c {
	case bodyAction, bodyState, bodyCalc, bodyCase:
		return true
	}
	return false
}

// pushBodyContext enters a body of the given notation and returns the function
// that leaves it.
func (p *Parser) pushBodyContext(c bodyContext) func() {
	depth := len(p.bodyCtx)
	p.bodyCtx = append(p.bodyCtx, c)
	return func() { p.bodyCtx = p.bodyCtx[:depth] }
}

// bodyContext returns the notation of the innermost body being parsed.
func (p *Parser) bodyContext() bodyContext {
	if len(p.bodyCtx) == 0 {
		return bodyOther
	}
	return p.bodyCtx[len(p.bodyCtx)-1]
}

// parseCheckpoint captures parser state for backtracking.
type parseCheckpoint struct {
	pos           int
	diagnosticLen int
	warningLen    int
	pendingSpan   source.Span
	hadPending    bool
}

// tokenWindow is how many consumed tokens the buffer keeps before compacting.
const tokenWindow = 64

// New creates a Parser for the given source file.
func New(sf *source.SourceFile) *Parser {
	// Models measure ~5 source bytes per non-trivia token; the window caps it.
	return &Parser{src: sf, lx: lexer.New(sf), buf: make([]lexer.Token, 0, min(sf.Len()/5+1, 2*tokenWindow))}
}

// fill ensures buf holds the token n positions ahead of the cursor (pulling
// from the lexer, skipping trivia and recording the notes and comments among
// it). The final EOF token is sticky (re-returned).
func (p *Parser) fill(n int) {
	p.compact()
	for len(p.buf) <= p.pos-p.base+n {
		tok := p.lx.Next()
		for tok.IsTrivia() || tok.Kind == lexer.RegularComment {
			if tr, ok := triviaOf(tok); ok {
				p.triv = append(p.triv, tr)
			}
			if tok.Unterminated {
				// Everything after the opener is inside it, so the declarations
				// that follow are not in the tree at all.
				p.error(tok.Span, "unterminated comment: missing */")
			}
			if tok.Kind == lexer.RegularComment {
				p.pendingComment = tok.Span
				p.hasPendingComment = true
			}
			if tok.Kind == lexer.EOF {
				break
			}
			tok = p.lx.Next()
		}
		if tok.BadEscape {
			p.reportBadEscapes(tok)
		}
		p.buf = append(p.buf, p.unreserved(tok))
		if tok.Kind == lexer.EOF {
			// keep EOF sticky: stop growing further with real tokens
			return
		}
	}
}

// reportBadEscapes reports each backslash escape in a quoted token outside the
// set its terminal admits (KerMLExpressions.xtext STRING_VALUE, UNRESTRICTED_NAME).
func (p *Parser) reportBadEscapes(tok lexer.Token) {
	terminal := "a string"
	if tok.Kind == lexer.UnrestrictedName {
		terminal = "an unrestricted name"
	}
	raw := p.src.Text(tok.Span)
	for _, sp := range lexer.InvalidEscapes(tok.Span, raw) {
		p.error(sp, fmt.Sprintf("invalid escape '%s' in %s: only \\b \\t \\n \\f \\r \\\" \\' \\\\ are admitted", p.src.Text(sp), terminal))
	}
}

// compact drops the consumed tokens no checkpoint can rewind to, keeping the
// previous token for lastEnd; dropping at least half at a time amortizes the copy.
func (p *Parser) compact() {
	drop := p.pos - p.base - 1
	if p.checkpoints > 0 || drop < tokenWindow || drop < len(p.buf)/2 {
		return
	}
	p.buf = p.buf[:copy(p.buf, p.buf[drop:])]
	p.base += drop
}

// triviaOf converts a note or comment token into the trivia recorded for it.
// Whitespace is most of a file's trivia and no consumer reads it, so it is not
// recorded.
func triviaOf(tok lexer.Token) (ast.Trivia, bool) {
	var k ast.TriviaKind
	switch tok.Kind {
	case lexer.SLNote:
		k = ast.TriviaLineNote
	case lexer.MLNote:
		k = ast.TriviaBlockNote
	case lexer.RegularComment:
		k = ast.TriviaComment
	default:
		return ast.Trivia{}, false
	}
	return ast.Trivia{Kind: k, Span: tok.Span}, true
}

// peek returns the current non-trivia token without consuming it.
func (p *Parser) peek() lexer.Token { return p.peekN(0) }

// peekN returns the token n positions ahead (0 = current). The buffered fast
// path is kept small enough to inline into the parser's hot accessors.
func (p *Parser) peekN(n int) lexer.Token {
	if i := p.pos - p.base + n; i < len(p.buf) {
		return p.buf[i]
	}
	return p.peekSlow(n)
}

// peekSlow pulls tokens from the lexer until the one n ahead is buffered.
func (p *Parser) peekSlow(n int) lexer.Token {
	p.fill(n)
	if i := p.pos - p.base + n; i < len(p.buf) {
		return p.buf[i]
	}
	return p.buf[len(p.buf)-1] // EOF (sticky)
}

// advance consumes and returns the current token.
func (p *Parser) advance() lexer.Token {
	if p.pos-p.base >= len(p.buf) {
		p.fill(0)
	}
	tok := p.buf[p.pos-p.base]
	if tok.Kind != lexer.EOF {
		p.pos++
	}
	return tok
}

// atEOF reports whether the current token is EOF.
func (p *Parser) atEOF() bool { return p.peek().Kind == lexer.EOF }

// Offset returns the source offset of the next unconsumed token, which is where
// a caller parsing a sequence of productions continues from. A node's span is
// not that position: a parenthesized expression's span is its contents.
func (p *Parser) Offset() int { return p.peek().Span.Offset }

// at reports whether the current token has the given kind.
func (p *Parser) at(k lexer.Kind) bool { return p.peek().Kind == k }

// atKeyword reports whether the current token is the given keyword literal.
func (p *Parser) atKeyword(kw string) bool {
	t := p.peek()
	return t.Kind == lexer.Keyword && t.KeywordID == kw
}

// acceptSufficientAll consumes the `all` of a declaration prefix
// (`isSufficient ?= 'all'`, KerML.xtext:325). SysML.xtext declares `all` only
// after `import`, so there the word names the declaration instead.
func (p *Parser) acceptSufficientAll() bool {
	if p.src.Kind() == source.KindSysML {
		return false
	}
	return p.acceptKeyword("all")
}

// peekIsKeyword reports whether the token n ahead of the cursor is the given
// keyword literal.
func (p *Parser) peekIsKeyword(n int, kw string) bool {
	t := p.peekN(n)
	return t.Kind == lexer.Keyword && t.KeywordID == kw
}

// accept consumes the current token if it matches kind, reporting success.
func (p *Parser) accept(k lexer.Kind) (lexer.Token, bool) {
	if p.at(k) {
		return p.advance(), true
	}
	return p.peek(), false
}

// acceptKeyword consumes the current token if it is the given keyword.
func (p *Parser) acceptKeyword(kw string) bool {
	if p.atKeyword(kw) {
		p.advance()
		return true
	}
	return false
}

// valueOperatorAt reports whether the token n ahead introduces a feature value.
func (p *Parser) valueOperatorAt(n int) bool {
	t := p.peekN(n)
	return t.Kind == lexer.Eq || t.Kind == lexer.ColonEq ||
		(t.Kind == lexer.Keyword && t.KeywordID == "default")
}

// valueOperator is a consumed feature value operator: its span from the first
// token through the last, and the `default` and `:=` forms it was written in.
type valueOperator struct {
	span      source.Span
	isDefault bool
	isInitial bool
}

// acceptValueOperator consumes a feature value operator (`=`, `:=`, or
// `default` followed by an optional `=` or `:=`).
func (p *Parser) acceptValueOperator() (valueOperator, bool) {
	if op, ok := p.accept(lexer.Eq); ok {
		return valueOperator{span: op.Span}, true
	}
	if op, ok := p.accept(lexer.ColonEq); ok {
		return valueOperator{span: op.Span, isInitial: true}, true
	}
	if p.atKeyword("default") {
		first := p.advance()
		last := first
		isInitial := false
		if op, ok := p.accept(lexer.Eq); ok {
			last = op
		} else if op, ok := p.accept(lexer.ColonEq); ok {
			last = op
			isInitial = true
		}
		span := source.Span{Offset: first.Span.Offset, Len: last.Span.End() - first.Span.Offset}
		return valueOperator{span: span, isDefault: true, isInitial: isInitial}, true
	}
	return valueOperator{}, false
}

func (p *Parser) parseUsageValue(u *ast.Usage) bool {
	op, ok := p.acceptValueOperator()
	if !ok {
		return false
	}
	u.ValueOperatorSpan = op.span
	u.ValueIsDefault = op.isDefault
	u.ValueIsInitial = op.isInitial
	u.Value = p.ParseExpression()
	return true
}

// expect consumes a token of the given kind or records a diagnostic at the
// current position and returns ok=false (without consuming).
func (p *Parser) expect(k lexer.Kind, msg string) (lexer.Token, bool) {
	if p.at(k) {
		return p.advance(), true
	}
	p.error(p.peek().Span, msg)
	return p.peek(), false
}

// expectSemicolon consumes the `;` ending a construct or reports it missing.
func (p *Parser) expectSemicolon(what string) bool {
	if p.accept2(lexer.Semicolon) {
		return true
	}
	p.missingTerminator("expected ';' after "+what, "missing ';' at end of "+what, p.insertSemicolonFix())
	return false
}

// expectBodyOrEnd consumes the `{` opening a construct's body, after its `;`
// form was not taken; neither carries a fix, since either would repair it.
func (p *Parser) expectBodyOrEnd(what string) bool {
	if p.accept2(lexer.LBrace) {
		return true
	}
	p.missingTerminator("expected '{' or ';' after "+what, "missing ';' at end of "+what)
	return false
}

// missingTerminator reports the terminator a construct lacks: on its last token when
// the next one starts a later line or ends the body, else on that unexpected token.
func (p *Parser) missingTerminator(unexpected, missing string, fixes ...quickfix.Fix) {
	next := p.peek()
	last, ok := p.lastToken()
	if !ok {
		p.errorWithFixes(next.Span, unexpected, fixes...)
		return
	}
	lines := p.src.Lines()
	switch {
	case next.Kind == lexer.RBrace, next.Kind == lexer.EOF,
		lines.PosAt(next.Span.Offset).Line > lines.PosAt(last.Span.End()).Line:
		p.errorWithFixes(last.Span, missing, fixes...)
	default:
		p.errorWithFixes(next.Span, unexpected, fixes...)
	}
}

// insertSemicolonFix is the edit that writes the missing `;` where the last
// consumed token ends.
func (p *Parser) insertSemicolonFix() quickfix.Fix {
	return quickfix.Fix{
		Title:     "Insert ';'",
		Edits:     []quickfix.Edit{quickfix.Insert(p.lastEnd(), ";")},
		Preferred: true,
	}
}

// lastToken returns the last consumed non-trivia token while the window holds it.
func (p *Parser) lastToken() (lexer.Token, bool) {
	if i := p.pos - p.base; i > 0 && i <= len(p.buf) {
		return p.buf[i-1], true
	}
	return lexer.Token{}, false
}

// lastEnd returns the end offset of the last consumed non-trivia token, or the
// start of the current one when nothing has been consumed.
func (p *Parser) lastEnd() int {
	if last, ok := p.lastToken(); ok {
		return last.Span.End()
	}
	return p.peek().Span.Offset
}

// error records a diagnostic that makes the parse ill-formed.
func (p *Parser) error(sp source.Span, msg string) {
	p.Diagnostics = append(p.Diagnostics, Diagnostic{Span: sp, Message: msg})
}

// errorWithFixes records an ill-formed-parse diagnostic that unambiguous edits
// resolve.
func (p *Parser) errorWithFixes(sp source.Span, msg string, fixes ...quickfix.Fix) {
	p.Diagnostics = append(p.Diagnostics, Diagnostic{Span: sp, Message: msg, Fixes: fixes})
}

// warn records a diagnostic for input that parses to the tree the author meant
// but is not well-formed SysML, under the code a consumer reports it by. It is
// kept apart from Diagnostics so that callers gating on a clean parse are not
// blocked by it.
func (p *Parser) warn(sp source.Span, msg, code string) {
	p.Warnings = append(p.Warnings, Diagnostic{Span: sp, Message: msg, Code: code})
}

// takeTrivia returns and clears the pending leading trivia.
func (p *Parser) takeTrivia() []ast.Trivia {
	t := p.triv
	p.triv = nil
	return t
}

// takePendingComment returns and clears the most recent regular-comment span.
func (p *Parser) takePendingComment() (source.Span, bool) {
	if !p.hasPendingComment {
		return source.Span{}, false
	}
	sp := p.pendingComment
	p.hasPendingComment = false
	return sp, true
}

// spanFrom builds a span from a start offset to the end of the previously
// consumed token region (current token's start).
func (p *Parser) spanFrom(start int) source.Span {
	end := p.peek().Span.Offset
	if end < start {
		end = start
	}
	return source.Span{Offset: start, Len: end - start}
}

// ParseFile parses the whole source as a RootNamespace (brace-less member list).
func (p *Parser) ParseFile() *ast.RootNamespace {
	start := p.peek().Span.Offset
	root := &ast.RootNamespace{}
	for !p.atEOF() {
		before := p.pos
		beforeOff := p.peek().Span.Offset
		root.Members = append(root.Members, p.parseMember())
		// Guarantee progress: if nothing was consumed, skip a token.
		if p.pos == before && p.peek().Span.Offset == beforeOff && !p.atEOF() {
			p.advance()
		}
	}
	root.NodeSpan = p.spanFrom(start)
	return root
}

// checkpoint captures current parser state for backtracking. The tokens it can
// rewind to stay buffered until release is called, whether or not it is restored.
func (p *Parser) checkpoint() parseCheckpoint {
	p.checkpoints++
	return parseCheckpoint{
		pos:           p.pos,
		diagnosticLen: len(p.Diagnostics),
		warningLen:    len(p.Warnings),
		pendingSpan:   p.pendingComment,
		hadPending:    p.hasPendingComment,
	}
}

// restore rewinds parser to a previous checkpoint, un-consuming the tokens the
// abandoned attempt read and dropping the findings it reported. Used for
// try-parse patterns.
//
// Trivia collected during the attempt is deliberately kept: the lexer yields
// each trivia token once, so dropping it would lose a comment from the tree.
func (p *Parser) restore(cp parseCheckpoint) {
	p.pos = cp.pos
	p.Diagnostics = p.Diagnostics[:cp.diagnosticLen]
	p.Warnings = p.Warnings[:cp.warningLen]
	p.pendingComment = cp.pendingSpan
	p.hasPendingComment = cp.hadPending
}

// release ends a checkpoint's hold on the token buffer; a try-parse defers it
// right after taking the checkpoint.
func (p *Parser) release() {
	p.checkpoints--
}
