package parser

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// parseConstraintCalcBody parses a constraint body that declares parameters,
// where a bare expression states a condition rather than a result.
func (p *Parser) parseConstraintCalcBody() []ast.Node {
	p.constraintCalcDepth++
	defer func() { p.constraintCalcDepth-- }()
	return p.parseCalcBody()
}

// parseCalcBody parses the body of a calc def/usage: its members, including the
// `return` that declares its result parameter and its trailing expression.
// Expects '{' already consumed.
func (p *Parser) parseCalcBody() []ast.Node {
	body := p.newBodyBuilder()
	p.calcBodyDepth++
	defer func() { p.calcBodyDepth-- }()

	// The mark applies to this body only, so a calculation nested in it reads
	// its own bare expression as a result again.
	constraintConditions := p.constraintCalcDepth > 0
	if constraintConditions {
		p.constraintCalcDepth--
		defer func() { p.constraintCalcDepth++ }()
	}

	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset

		// A calculation body carries the members of an action body
		// (SysML.xtext CalculationBodyItem), a member-attached `then` among them.
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}

		// A constraint body that declares parameters is read here, so its
		// asserted conditions are members of this body too.
		if p.atConstraintCondition() {
			body.add(p.parseConstraintMember())
			continue
		}

		// `return` declares the result parameter (SysML.xtext ReturnParameterMember).
		if p.isResultKeyword() {
			body.add(p.parseResultMember())
		} else if p.atCalcBodyItem() {
			body.add(p.parseActionMember())
		} else {
			// An expression start that does not declare a member is the body's
			// trailing expression. A `var`-prefixed declaration is a member, not
			// the value of an expression naming a feature `var` (KerML BasicFeaturePrefix).
			if p.atUnaryExprStart() && !p.atNamedCalcMember() && !p.atVarDeclaration() {
				// In a constraint body a bare expression is a condition the
				// constraint states, not a calculated result.
				if constraintConditions {
					body.add(p.parseConstraintMember())
				} else {
					body.add(p.ParseExpression())
				}
			} else {
				// Parse as generic body member (parameters, etc.)
				body.add(p.parseBodyMember())
			}
		}

		// Guard against infinite loop
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}

	p.expect(lexer.RBrace, "expected '}' after calc body")
	return body.finish()
}

// atNamedCalcMember reports whether the cursor begins a kind-less member declaration
// (SysML.xtext DefaultReferenceUsage) rather than an expression or a `done;` node.
func (p *Parser) atNamedCalcMember() bool {
	if !p.atName() && !p.at(lexer.Lt) {
		return false
	}
	if _, ok := p.atActionNodeWord(); ok {
		return false
	}
	cp := p.checkpoint()
	p.parseIdentification()
	t := p.peek()
	declared := p.atFeatureSpecialization() || p.at(lexer.LBracket) || p.at(lexer.LBrace) || p.at(lexer.Semicolon) ||
		p.valueOperatorAt(0) || (t.Kind == lexer.Keyword && !wordBinaryOpKeywords[t.KeywordID])
	p.restore(cp)
	p.release()
	return declared
}

// parseActionBodyMixed parses the body of an action or state node: both
// declarations and behavioral statements. Expects '{' already consumed.
// Syntax: { in item x; action nested {...}; first nested then ...; flow ...; }
func (p *Parser) parseActionBodyMixed() []ast.Node {
	body := p.newBodyBuilder()

	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset

		// A member-attached `then` sequences the members either side of it, so
		// the keyword is taken here and the member it prefixes read next time
		// round, by whichever branch below that member needs (see
		// succession.go).
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}

		// Try parsing as direction parameter (in/out/inout item/accept/via)
		if p.isDirectionKeyword() {
			body.add(p.parseDirectionParameter())
			continue
		}

		// Check for nested action DECLARATION (action name : Type {...} or action name {...})
		// vs behavioral action node (action ref or action {expr})
		if p.atKeyword("action") {
			// Lookahead to distinguish:
			// - action <id> : Type { ... } = declaration (typing)
			// - action <id> { stmts } = declaration (multi-statement body)
			// - action <id> { expr } = behavioral node (inline expression)
			// - action { expr } = behavioral node
			// - action <id>; = behavioral node (reference)
			// Check for typing colon OR declaration-like body content
			tok1 := p.peekN(1)
			// An accept node, however it is identified: `action accept …`
			// naming no node of its own, `action nm accept …`, and the short
			// name and name both.
			if p.atAcceptNode() {
				body.add(p.parseBodyMember())
				continue
			}
			if tok1.Kind == lexer.Identifier || tok1.Kind == lexer.Keyword {
				tok2 := p.peekN(2)
				// If colon after name → definitely declaration (typing)
				if tok2.Kind == lexer.Colon {
					body.add(p.parseBodyMember())
					continue
				}
				// If 'accept' keyword after name → declaration (accept action)
				// Pattern: action <name> accept <param> : Type [via <port>];
				if tok2.Kind == lexer.Keyword && tok2.KeywordID == "accept" {
					body.add(p.parseBodyMember())
					continue
				}
				// If behavioral keyword after name → declaration with inline statement
				// Pattern: action <name> send <msg> to <target>;
				//          action <name> perform <ref>;
				if tok2.Kind == lexer.Keyword && (tok2.KeywordID == "send" || tok2.KeywordID == "terminate" ||
					tok2.KeywordID == "perform" || tok2.KeywordID == "bind" || tok2.KeywordID == "assign") {
					body.add(p.parseBodyMember())
					continue
				}
				// If brace after name, peek inside
				if tok2.Kind == lexer.LBrace && p.startsActionBodyItem(3) {
					body.add(p.parseBodyMember())
					continue
				}
			}
			// `action { <statements> }` is the anonymous ActionBodyParameter a loop
			// or branch body is written as (SysML.xtext ActionBodyParameter), not the
			// one expression an `action { <expr> }` node computes.
			if tok1.Kind == lexer.LBrace && p.startsActionBodyItem(2) {
				body.add(p.parseBodyMember())
				continue
			}
			// Otherwise: treat as behavioral action node
			body.add(p.parseActionMember())
			continue
		}

		if p.isBehavioralKeyword() {
			body.add(p.parseActionMember())
			continue
		}

		// Try parsing as body member (nested declarations)
		body.add(p.parseBodyMember())

		// Ensure progress
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.error(p.peek().Span, msgExpectedBodyMember)
			p.advance()
		}
	}

	p.expect(lexer.RBrace, "expected '}' after action body")
	return body.finish()
}

// parseNodeBody reads the body an action or state node production ends in
// (SysML.xtext ActionBody, StateUsageBody): a braced member list, or ';'. Every
// node that ends in one reads it here, so a body is taken wherever the notation
// allows one.
func (p *Parser) parseNodeBody(start int, what string) ([]ast.Node, bool) {
	if p.at(lexer.LBrace) {
		p.advance() // consume '{'
		defer p.pushBodyContext(bodyAction)()
		return p.parseActionBodyMixed(), true
	}
	p.expectStatementEnd(start, "expected ';' or '{' after "+what)
	return nil, false
}

// parseNodeDeclaration reads the optional declaration a control node carries
// (SysML.xtext ControlNode: `UsageDeclaration?`), which may name the node with
// any name a usage takes, an unrestricted one (`decide 'test x';`) included.
func (p *Parser) parseNodeDeclaration() (string, source.Span) {
	if !p.atName() && !p.at(lexer.Lt) {
		return "", source.Span{}
	}
	id := p.parseIdentification()
	if id.Name != "" {
		return id.Name, id.NameSpan
	}
	return id.ShortName, id.ShortNameSpan
}

// parseChainedName parses the name a behavior member refers to: a qualified
// name whose segments may be chained with '.' as well as with '::', which is how
// the notation reaches a nested state or a port of a part (`S2.S3`,
// `tellu.APIS_HTTP`).
func (p *Parser) parseChainedName() *ast.QualifiedName {
	qn := p.parseQualifiedNameRelaxed()
	if qn == nil {
		return nil
	}
	for p.at(lexer.Dot) {
		next := p.peekN(1)
		if next.Kind != lexer.Identifier && next.Kind != lexer.UnrestrictedName && next.Kind != lexer.Keyword {
			break
		}
		p.advance() // consume '.'
		seg, ok := p.parseNameSegmentRelaxed()
		if !ok {
			break
		}
		seg.Chained = true
		qn.Parts = append(qn.Parts, seg)
		qn.NodeSpan = p.spanFrom(qn.NodeSpan.Offset)
	}
	return qn
}

// isDirectionKeyword checks if current token is in/out/inout
func (p *Parser) isDirectionKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	kw := p.peek().KeywordID
	return kw == "in" || kw == "out" || kw == "inout"
}

// parameterKindKeywords are the kind keywords a directed parameter may state; any
// other keyword there names the parameter instead.
var parameterKindKeywords = map[string]ast.UsageKind{
	"item":       ast.UsageItem,
	"feature":    ast.UsagePart,
	"port":       ast.UsagePort,
	"part":       ast.UsagePart,
	"attribute":  ast.UsageAttribute,
	"occurrence": ast.UsageOccurrence,
	"action":     ast.UsageAction,
	"calc":       ast.UsageCalc,
}

// parameterKindKeyword reports whether parseDirectionParameter reads the token as
// the parameter's kind.
func parameterKindKeyword(t lexer.Token) bool {
	if t.Kind != lexer.Keyword {
		return false
	}
	_, ok := parameterKindKeywords[t.KeywordID]
	return ok
}

// parseDirectionParameter parses: <direction> [ref] [<kind>] [<name>] [: <type>] [= <value>];
// Examples: in item scene; out feature x; in ref item x : Foo = bar; in item; in target;
// Kind keyword is optional - defaults to generic feature
func (p *Parser) parseDirectionParameter() ast.Node {
	start := p.peek().Span.Offset

	// Parse direction (in/out/inout)
	dirTok, _ := p.expect(lexer.Keyword, "expected direction keyword")
	var direction ast.FeatureDirection
	switch dirTok.KeywordID {
	case "in":
		direction = ast.DirIn
	case "out":
		direction = ast.DirOut
	case "inout":
		direction = ast.DirInOut
	default:
		direction = ast.DirNone
	}

	// A direction prefixes the feature it applies to, so a parameter declaring
	// nothing at all is that feature missing (SysML.xtext FeatureDirection).
	if p.at(lexer.Semicolon) || p.at(lexer.RBrace) {
		p.error(p.peek().Span, "expected a feature after '"+dirTok.KeywordID+"': write `"+dirTok.KeywordID+" <name> : <Type>`")
	}

	// Check for optional 'ref' modifier
	isRef := false
	if p.atKeyword("ref") {
		p.advance()
		isRef = true
	}

	// Check for optional individual/portion/event modifiers
	isIndividual := false
	portion := ast.PortionNone
	isEvent := false
	if p.atKeyword("individual") {
		p.advance()
		isIndividual = true
	} else if p.atKeyword("snapshot") {
		p.advance()
		portion = ast.PortionSnapshot
	} else if p.atKeyword("timeslice") {
		p.advance()
		portion = ast.PortionTimeslice
	} else if p.atKeyword("event") {
		p.advance()
		isEvent = true
	}

	mods := featureMods{
		direction:    direction,
		isReference:  isRef,
		isIndividual: isIndividual,
		portion:      portion,
		isEvent:      isEvent,
	}
	p.warnAmbiguousModifierKind(mods, parameterKindKeyword)

	// A parameter's kind keyword is optional; a lone occurrence modifier declares the
	// kind, and with no keyword at all the parameter is the same kindless
	// reference usage as a keyword-less declaration outside a parameter list
	// (SysML.xtext DefaultReferenceUsage, SysML v2 §7.6).
	kind, _ := modifierImpliedKind(mods)
	// Any other keyword is left as the parameter's name (kind stays default).
	if p.at(lexer.Keyword) {
		if k, ok := parameterKindKeywords[p.peek().KeywordID]; ok {
			kind = k
			p.advance() // consume kind keyword
		}
	}

	// Optional name (can be anonymous: "in item;"). A keyword here names the
	// parameter, as the stdlib's `in 'type': Anything;` does; `ordered`/`nonunique`
	// follow a nameless parameter instead, so they stop the name.
	var ident ast.Identification
	if p.atNameOrKeyword() || p.at(lexer.Lt) {
		ident = p.parseIdentificationStopping("ordered", "nonunique")
	}

	// Create Usage node with direction
	usage := &ast.Usage{
		Kind:         kind,
		Ident:        ident,
		IsReference:  isRef,
		Direction:    direction,
		IsEvent:      isEvent,
		IsIndividual: isIndividual,
		Portion:      portion,
	}

	// A parameter specializes and states its multiplicity as any usage does
	// (`in x : T[1] redefines y`, `in x[1] : T`), so it shares the usage loop.
	p.parseFeatureSpecializationPart(usage)

	// Optional value (= expr, := expr, or default [=] expr)
	valueOp, hasValue := p.acceptValueOperator()
	if hasValue {
		usage.Value = p.ParseExpression()
	}
	usage.ValueOperatorSpan = valueOp.span
	usage.ValueIsDefault = valueOp.isDefault
	usage.ValueIsInitial = valueOp.isInitial

	// Optional body or semicolon
	if p.accept2(lexer.Semicolon) {
		usage.HasBody = false
	} else if p.at(lexer.LBrace) {
		p.advance() // consume '{'
		// Parse body members generically
		leave := p.pushBodyContext(bodyOther)
		for !p.at(lexer.RBrace) && !p.atEOF() {
			m := p.parseBodyMember()
			if m != nil {
				usage.Members = append(usage.Members, m)
			}
		}
		leave()
		p.expect(lexer.RBrace, "expected '}'")
		usage.HasBody = true
	} else {
		p.error(p.peek().Span, "expected ';' or '{' after parameter")
	}
	usage.NodeSpan = p.spanFrom(start)

	// Wrap in Membership
	membership := &ast.Membership{
		Member: usage,
	}
	membership.NodeSpan = usage.NodeSpan

	return membership
}

// parseActionMember parses one action member: node, edge, or nested declaration.
func (p *Parser) parseActionMember() ast.Node {
	start := p.peek().Span.Offset

	// A `return` reached in a statement position of a calculation body declares
	// the result parameter, as one among its members does.
	if p.calcBodyDepth > 0 && p.isResultKeyword() {
		return p.parseResultMember()
	}

	// An accept node is an action node (SysML.xtext ActionNode), so it stands
	// wherever a statement does: `accept e : E;`, `then action a accept e : E { … }`.
	if p.atAcceptNode() {
		return p.parseBodyMember()
	}

	// The words of our own node notation are names the lexer does not reserve,
	// so they are matched by the shape around them (see notation.go).
	if _, ok := p.atActionNodeWord(); ok {
		return p.parseFinalNode(p.advance())
	}

	// A `first` end reached through a feature chain is a SuccessionAsUsage
	// rather than an initial node, whose member element is a plain name.
	if p.atChainedFirstSuccession() {
		return p.parseSuccessionAsUsage(start)
	}

	// Try general declaration first (nested actions, features, etc.)
	if node := p.tryParseDeclaration(); node != nil {
		return node
	}

	// A member only some body kind owns is read by the body member parser,
	// which reports it when this body is not that kind.
	if _, owned := ownedMembers[p.peek().KeywordID]; owned && p.at(lexer.Keyword) {
		return p.parseBodyMember()
	}

	// Handle doc keyword specially (parseDocumentation consumes it)
	if p.atKeyword("doc") {
		return p.parseDocumentation(start)
	}

	// Check for keyword dispatch
	if tok, ok := p.accept(lexer.Keyword); ok {
		kw := tok.KeywordID
		switch kw {
		case "first":
			return p.parseInitialNode(tok)
		case "fork":
			return p.parseForkNode(tok)
		case "join":
			return p.parseJoinNode(tok)
		case "merge":
			return p.parseMergeNode(tok)
		case "decide":
			return p.parseDecisionNode(tok)
		case "action":
			return p.parseActionExecutionNode(tok)
		case "then":
			return p.parseSuccessionEdge(tok, true)
		case "assign":
			return p.parseAssignmentAction(tok)
		case "perform":
			return p.parsePerformAction(tok)
		case "while":
			return p.parseWhileLoopAction(tok)
		case "loop":
			return p.parseLoopAction(tok)
		case "for":
			return p.parseForAction(tok)
		case "if":
			return p.parseIfAction(tok)
		case "else":
			return p.parseDefaultTargetSuccession(tok)
		case "send":
			return p.parseSendStatement(tok)
		case "terminate":
			return p.parseTerminateStatement(tok)
		default:
			// Unknown keyword - try as general body member
			// Restore checkpoint since we consumed keyword
			p.error(tok.Span, "unknown action keyword: "+kw)
			en := &ast.ErrorNode{Message: "unknown action keyword: " + kw}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
	}

	// Graceful fallback: try parseBodyMember for remaining constructs
	if node := p.parseBodyMember(); node != nil {
		return node
	}

	// Last resort: report error but don't crash
	p.error(p.peek().Span, "expected action member")
	en := &ast.ErrorNode{Message: "expected action member"}
	if !p.atEOF() && !p.at(lexer.RBrace) {
		p.advance() // ensure progress
	}
	en.NodeSpan = p.spanFrom(start)
	return en
}

// Action node parsers and succession edges.

func (p *Parser) parseInitialNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	var name string
	var nameSpan source.Span

	// The name refers to a member, so it is kept as the name itself: an
	// unrestricted name without its quotes, as a qualified name segment is.
	if seg, ok := p.parseNameSegmentRelaxed(); ok {
		name = seg.Text
		nameSpan = seg.Span
	}

	// Check for succession edge continuation: first X [if <expr>] then Y;
	var guard ast.Node
	if p.atKeyword("if") {
		p.advance() // consume 'if'
		guard = p.ParseExpression()
	}

	var successor *ast.QualifiedName
	if p.atKeyword("then") {
		p.advance() // consume 'then'
		successor = p.parseChainedName()
	}

	// If guard present but no then, error
	if guard != nil && successor == nil {
		p.error(p.peek().Span, "expected 'then' after guard condition")
	}

	// The succession a `first … then …` states ends in a body of its own
	// (SysML.xtext ActionTargetSuccession, which ends in UsageBody).
	members, hasBody := p.parseNodeBody(start, "initial node")

	node := &ast.InitialNode{
		Name:      name,
		NameSpan:  nameSpan,
		Successor: successor,
		Guard:     guard,
		Members:   members,
		HasBody:   hasBody,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseFinalNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	if p.atNameOrKeyword() {
		nameToken := p.peek()
		p.error(nameToken.Span, "a final node declares no name; a succession names the `done` library feature")
		p.advance()
		p.expect(lexer.Semicolon, "expected ';' after final node")
		en := &ast.ErrorNode{Message: "a final node declares no name; a succession names the `done` library feature"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	p.expect(lexer.Semicolon, "expected ';' after final node")

	node := &ast.FinalNode{}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseForkNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	name, nameSpan := p.parseNodeDeclaration()
	members, hasBody := p.parseNodeBody(start, "fork node")

	node := &ast.ForkNode{
		Name:     name,
		NameSpan: nameSpan,
		Members:  members,
		HasBody:  hasBody,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseJoinNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	name, nameSpan := p.parseNodeDeclaration()
	members, hasBody := p.parseNodeBody(start, "join node")

	node := &ast.JoinNode{
		Name:     name,
		NameSpan: nameSpan,
		Members:  members,
		HasBody:  hasBody,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseMergeNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	name, nameSpan := p.parseNodeDeclaration()
	members, hasBody := p.parseNodeBody(start, "merge node")

	node := &ast.MergeNode{
		Name:     name,
		NameSpan: nameSpan,
		Members:  members,
		HasBody:  hasBody,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseDecisionNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	name, nameSpan := p.parseNodeDeclaration()
	members, hasBody := p.parseNodeBody(start, "decision node")

	node := &ast.DecisionNode{
		Name:     name,
		NameSpan: nameSpan,
		Members:  members,
		HasBody:  hasBody,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseActionExecutionNode(tok lexer.Token) ast.Node {
	// Syntax:
	//   action [name] actionRef ;
	//   action [name] { expression } ;
	start := tok.Span.Offset
	trivia := p.takeTrivia()

	var name string
	var actionRef *ast.QualifiedName
	var expression ast.Node

	// Disambiguate: name vs ref, inline vs reference mode
	if p.at(lexer.LBrace) {
		// Inline mode, no name: action { expr };
		p.advance() // consume '{'
		expression = p.ParseExpression()
		_, ok := p.expect(lexer.RBrace, msgExpectedActionBrace)
		if !ok {
			return &ast.ErrorNode{
				NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
				Message:  msgExpectedActionBrace,
			}
		}
	} else if p.at(lexer.Identifier) {
		// Could be:
		// - action name { expr }; (name + inline)
		// - action name ref; (name + ref)
		// - action ref; (ref only)
		// Use lookahead to decide
		nextTok := p.peekN(1)
		if nextTok.Kind == lexer.LBrace {
			// name + inline: action name { expr };
			nameToken := p.peek()
			name = p.src.Text(nameToken.Span)
			p.advance()
			p.advance() // consume '{'
			expression = p.ParseExpression()
			_, ok := p.expect(lexer.RBrace, msgExpectedActionBrace)
			if !ok {
				return &ast.ErrorNode{
					NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
					Message:  msgExpectedActionBrace,
				}
			}
		} else if nextTok.Kind == lexer.Identifier || nextTok.Kind == lexer.ColonColon || nextTok.Kind == lexer.Keyword {
			// Could be name + ref OR just ref (qualified name)
			// Also handle keywords as refs (e.g., action stop terminate;)
			// Parse first identifier
			firstIdToken := p.peek()
			firstIdSpan := firstIdToken.Span
			firstId := p.src.Text(firstIdSpan)
			p.advance()

			// Check what follows
			if p.at(lexer.ColonColon) {
				// It's a qualified name starting with firstId (no separate name)
				// Build QualifiedName manually since we consumed first part
				parts := []ast.NameSegment{{Text: firstId, Span: firstIdSpan}}
				for p.at(lexer.ColonColon) {
					p.advance() // consume '::'
					if !p.at(lexer.Identifier) {
						return &ast.ErrorNode{
							NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
							Message:  "expected identifier after '::'",
						}
					}
					seg := p.peek()
					parts = append(parts, ast.NameSegment{Text: p.src.Text(seg.Span), Span: seg.Span})
					p.advance()
				}
				actionRef = &ast.QualifiedName{Parts: parts}
				actionRef.NodeSpan = p.spanFrom(firstIdSpan.Offset)
			} else if p.at(lexer.Identifier) || p.at(lexer.Keyword) {
				// firstId is name, what follows is actionRef (identifier or keyword)
				name = firstId
				if p.at(lexer.Keyword) {
					// Allow keywords as action refs (e.g., 'terminate')
					kw := p.peek()
					actionRef = &ast.QualifiedName{
						Parts: []ast.NameSegment{{Text: kw.KeywordID, Span: kw.Span}},
					}
					actionRef.NodeSpan = kw.Span
					p.advance()
				} else {
					actionRef = p.parseQualifiedName()
				}
			} else {
				// firstId is a simple (non-qualified) actionRef
				actionRef = &ast.QualifiedName{
					Parts: []ast.NameSegment{{Text: firstId, Span: firstIdSpan}},
				}
				actionRef.NodeSpan = firstIdSpan
			}
		} else {
			// Single identifier followed by something else (likely ';')
			idToken := p.peek()
			actionRef = &ast.QualifiedName{
				Parts: []ast.NameSegment{{Text: p.src.Text(idToken.Span), Span: idToken.Span}},
			}
			actionRef.NodeSpan = idToken.Span
			p.advance()
		}
	} else {
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
			Message:  "expected action reference or '{' after 'action'",
		}
	}

	p.expect(lexer.Semicolon, "expected ';' after action execution node")

	node := &ast.ActionExecutionNode{
		Name:       name,
		ActionRef:  actionRef,
		Expression: expression,
	}
	node.NodeSpan = p.spanFrom(start)
	node.SetLeadingTrivia(trivia)
	return node
}

// atChainedThen reports whether the parser is at a `then` chaining the preceding
// member to a keyword-introduced declaration or statement (`then action b {...}`,
// `then send s to t;`) rather than one starting a named succession edge
// (`succession first a then b;`) over members of the enclosing body.
func (p *Parser) atChainedThen() bool {
	return p.atKeyword("then") && p.peekN(1).Kind == lexer.Keyword
}

// atNamespaceSuccession reports whether the parser is at a `then` chaining to a
// following declaration (`action a {...} then action b {...}`) rather than one
// chaining a behavioral statement or starting a succession edge member.
func (p *Parser) atNamespaceSuccession() bool {
	if !p.atChainedThen() {
		return false
	}
	next := p.peekN(1)
	switch next.KeywordID {
	case "public", "private", "protected":
		return true
	}
	if _, isDef := p.definitionKind(next.KeywordID); isDef {
		return true
	}
	_, isUsage := p.usageKind(next.KeywordID)
	return isUsage
}

// startsActionBodyItem reports whether the token n ahead, the first inside a
// braced action body, begins an action body item (SysML.xtext ActionBodyItem)
// rather than an expression. Only words that cannot begin an expression qualify,
// so `done` counts only in the node shape (see notation.go).
func (p *Parser) startsActionBodyItem(n int) bool {
	if p.peekN(n).Kind == lexer.Identifier {
		_, ok := p.actionNodeWordAt(n)
		return ok
	}
	if p.peekN(n).Kind != lexer.Keyword {
		return false
	}
	switch p.peekN(n).KeywordID {
	case "in", "out", "inout",
		"action", "part", "item", "flow", "doc", "state", "port", "attribute",
		"perform", "send", "assign", "accept", "terminate",
		"first", "then", "fork", "join", "merge", "decide",
		"while", "loop", "for":
		return true
	}
	return false
}

// startsInlineSuccessionStatement reports whether tok, the token after a `then`,
// starts an inline statement succession (`then assign x := 1;`) rather than a
// named edge (`succession first source then target;`) over members of the enclosing body.
func startsInlineSuccessionStatement(tok lexer.Token) bool {
	if tok.Kind != lexer.Keyword {
		return false
	}
	switch tok.KeywordID {
	// `loop` and `for` head the same action node forms `while` does
	// (SysML.xtext WhileLoopNode, ForLoopNode), so a `then` before one chains a
	// statement rather than naming an edge end.
	case "assign", "perform", "while", "loop", "for", "if", "action", "accept":
		return true
	}
	return false
}

// parseSuccessionEdge parses implicit-source targets and inline statements.
// allowBody admits the UsageBody of an ActionTargetSuccession (SysML.xtext:1698).
func (p *Parser) parseSuccessionEdge(tok lexer.Token, allowBody bool) ast.Node {
	start := tok.Span.Offset

	// Check if this is inline statement succession (then followed by behavioral keyword)
	// Pattern: then assign x := 1; OR then perform foo;
	if startsInlineSuccessionStatement(p.peek()) {
		return p.parseActionMember()
	}

	// Otherwise, parse as named edge: then [source] target;
	// If only one name before semicolon, it's the target (source implicit)
	first := p.parseQualifiedNameRelaxed() // allow keywords like 'fork', 'join'

	// Check if there's a second name (explicit source + target)
	var source, target *ast.QualifiedName
	twoEnded := false
	if !p.at(lexer.Semicolon) && !p.atKeyword("if") && (p.at(lexer.Identifier) || p.at(lexer.Keyword)) {
		// Two-name form: succession first source then target;
		source = first
		target = p.parseQualifiedNameRelaxed()
		twoEnded = true
	} else {
		// One-name form: then target; (source implicit)
		source = &ast.QualifiedName{} // empty source
		target = first
	}

	if twoEnded {
		const msg = "a succession names both ends as `first <source> then <target>`"
		p.error(first.Span(), msg)
		for !p.at(lexer.Semicolon) && !p.atEOF() {
			p.advance()
		}
		p.expect(lexer.Semicolon, "expected ';' after succession edge")
		en := &ast.ErrorNode{Message: msg}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	// Check for optional guard
	if p.acceptKeyword("if") {
		// 'if' keyword already consumed
		guard := p.ParseExpression()

		p.expect(lexer.Semicolon, "expected ';' after control flow edge")

		node := &ast.ControlFlowEdge{
			NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
			Source:   source,
			Target:   target,
			Guard:    guard,
		}
		return node
	}

	// An action target succession ends in a UsageBody (SysML.xtext:1698).
	var members []ast.Node
	hasBody := false
	if allowBody && p.accept2(lexer.LBrace) {
		hasBody = true
		leave := p.pushBodyContext(bodyAction)
		members = p.parseActionBodyMixed()
		leave()
	} else {
		p.expect(lexer.Semicolon, "expected ';' after succession edge")
	}

	node := &ast.SuccessionEdge{
		NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
		Source:   source,
		Target:   target,
		Members:  members,
		HasBody:  hasBody,
	}
	return node
}

// expectStatementEnd terminates the behavioral statement that starts at start.
// Only a statement written as a transition's `do` effect is ended by the
// transition itself, either by its next clause or by its own ';'; elsewhere a
// missing ';' stays a syntax error.
func (p *Parser) expectStatementEnd(start int, msg string) {
	if p.atTransitionEffectStatement(start) && p.atEffectEnd() {
		return
	}
	if p.effectDepth > 0 && (p.atKeyword("then") || p.atKeyword("if") || p.atKeyword("do")) {
		return
	}
	p.expect(lexer.Semicolon, msg)
}

// atTransitionEffectStatement reports whether the member starting at start is the
// innermost transition's `do` effect rather than a statement nested in its body.
func (p *Parser) atTransitionEffectStatement(start int) bool {
	return p.effectDepth > 0 && p.effectStmtStart == start
}

// atEffectEnd reports whether the cursor is where a transition's effect ends: at
// the transition's own ';', or at the clause written after the effect.
func (p *Parser) atEffectEnd() bool {
	return p.at(lexer.Semicolon) || p.atKeyword("then") || p.atKeyword("if") || p.atKeyword("do")
}

// enterTransitionEffect records that the statement at the cursor is a
// transition's `do` effect and returns the function that leaves it.
func (p *Parser) enterTransitionEffect() func() {
	savedDepth, savedStart := p.effectDepth, p.effectStmtStart
	p.effectDepth++
	p.effectStmtStart = p.peek().Span.Offset
	return func() { p.effectDepth, p.effectStmtStart = savedDepth, savedStart }
}

// parseAssignmentAction parses: assign target := value;
func (p *Parser) parseAssignmentAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// The target names the feature assigned (SysML.xtext TargetParameter ends
	// in a FeatureChainMember); an expression there is an error.
	target := p.ParseExpression()
	if _, failed := target.(*ast.ErrorNode); target != nil && !failed && !namesFeature(target) {
		const msg = "an assignment target names a feature or a feature chain, not an expression; " +
			"assign to the feature that holds the value"
		p.error(target.Span(), msg)
		en := &ast.ErrorNode{Message: msg}
		en.NodeSpan = target.Span()
		target = en
	}

	// Expect ':=' operator
	if !p.at(lexer.ColonEq) {
		p.error(p.peek().Span, "expected ':=' after assignment target")
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
			Message:  "expected ':=' after assignment target",
		}
	}
	p.advance() // consume ':='

	// Parse value expression
	value := p.ParseExpression()

	p.expectStatementEnd(start, "expected ';' after assignment")

	node := &ast.AssignmentActionNode{
		Target: target,
		Value:  value,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parsePerformAction parses: perform action;
func (p *Parser) parsePerformAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// Parse action reference (qualified name or invocation)
	actionRef := p.ParseExpression()

	p.expectStatementEnd(start, "expected ';' after perform statement")

	node := &ast.PerformActionNode{
		ActionRef: actionRef,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseActionBodyParameter parses the `action [<name>] { <items> }` form of an
// ActionBodyParameter (SysML.xtext), with the `action` keyword at the cursor.
// The parameter stays an action usage member so a name it declares keeps
// scoping the body: `loop action charging { … } until charging.done`.
func (p *Parser) parseActionBodyParameter() []ast.Node {
	m := p.parseBodyMember()
	if m == nil {
		return nil
	}
	// The usage is marked as the body itself, so lowering makes its members the
	// block's statements whether or not it was given a name.
	member := m
	if membership, ok := member.(*ast.Membership); ok {
		member = membership.Member
	}
	if usage, ok := member.(*ast.Usage); ok && usage.Kind == ast.UsageAction {
		usage.IsBodyParameter = true
	}
	return []ast.Node{m}
}

// parseWhileLoopAction parses `while <condition> <action-body> ['until' <c>;']`
// (SysML.xtext WhileLoopNode).
func (p *Parser) parseWhileLoopAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// Parse condition expression
	condition := p.ParseExpression()

	var body []ast.Node
	if p.atKeyword("action") {
		body = p.parseActionBodyParameter()
	} else {
		if !p.at(lexer.LBrace) {
			p.error(p.peek().Span, "expected '{' after while condition")
			return &ast.ErrorNode{
				NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
				Message:  "expected '{' after while condition",
			}
		}
		p.advance() // consume '{'

		leave := p.pushBodyContext(bodyAction)
		for !p.at(lexer.RBrace) && !p.atEOF() {
			body = append(body, p.parseActionMember())
		}
		leave()

		p.expect(lexer.RBrace, "expected '}' after while body")
	}

	// A `while` loop may also carry an `until` clause, tested after each
	// iteration (SysML.xtext WhileLoopNode).
	var until ast.Node
	if p.acceptKeyword("until") {
		until = p.ParseExpression()
		p.expect(lexer.Semicolon, "expected ';' after 'until' condition")
	}

	node := &ast.WhileLoopActionNode{
		Kind:      ast.LoopWhile,
		Condition: condition,
		Until:     until,
		Body:      body,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseLoopAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// Loop syntax: `loop { <body> } until <condition>;`, or the unbraced
	// `loop <body-members> until <condition>;`. A braced body ends at its own
	// '}', so the members following the loop stay with the enclosing body.
	// Body can contain action declarations, succession, etc.
	// Parse body as mixed content (declarations + behavioral statements)
	_, braced := p.accept(lexer.LBrace)
	var body []ast.Node

	// The unbraced body is an ActionBodyParameter: `loop action [<name>] { … }`.
	if !braced && p.atKeyword("action") {
		body = p.parseActionBodyParameter()
	}

	// Parse loop body members until 'until' keyword or closing brace
	leave := p.pushBodyContext(bodyAction)
	for !p.atKeyword("until") && !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset

		// Try direction parameters first
		if p.isDirectionKeyword() {
			body = append(body, p.parseDirectionParameter())
			continue
		}

		// Try declarations (action/part/etc)
		if p.atDefUsageStart() {
			m := p.parseBodyMember()
			if m != nil {
				body = append(body, m)
			}
			// Check if no progress (prevent infinite loop)
			if p.peek().Span.Offset == before {
				p.advance()
			}
			continue
		}

		// Parse behavioral statements
		body = append(body, p.parseActionMember())

		// Ensure progress
		if p.peek().Span.Offset == before {
			p.advance()
		}
	}
	leave()

	if braced {
		p.expect(lexer.RBrace, "expected '}' after loop body")
	}

	// Parse optional 'until' condition
	var condition ast.Node
	if p.acceptKeyword("until") {
		condition = p.ParseExpression()
	}

	// An `until` clause is terminated by a semicolon in either form; a braced
	// body without one is already complete.
	if condition != nil || !braced {
		p.expect(lexer.Semicolon, "expected ';' after loop")
	} else {
		p.accept(lexer.Semicolon)
	}

	// The condition an `until` clause carries is tested after each iteration,
	// which is what LoopUntil records.
	node := &ast.WhileLoopActionNode{
		Kind:      ast.LoopUntil,
		Condition: condition,
		Body:      body,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseForAction parses: for <variable> in <collection> { <body> }
func (p *Parser) parseForAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// The loop variable is a usage declaration (SysML.xtext ForVariableDeclaration), so
	// a keyword may name it — `in` excepted, since it ends the declaration.
	variable := p.parseIdentificationStopping("in")
	if variable.Name == "" && variable.ShortName == "" {
		p.error(p.peek().Span, "expected variable name after 'for'")
		en := &ast.ErrorNode{Message: "expected variable name"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	// The variable is a full UsageDeclaration, so it may state its type before
	// `in` (`for n : ScalarValues::Integer in (1, 2, 3)`).
	variableRels := p.parseRelationships(true)

	// Expect 'in' keyword
	if !p.acceptKeyword("in") {
		p.error(p.peek().Span, "expected 'in' keyword after for variable")
		en := &ast.ErrorNode{Message: "expected 'in'"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	// Parse collection expression
	collection := p.ParseExpression()

	// Parse body as an action body parameter (SysML.xtext ForLoopNode).
	if p.atKeyword("action") {
		node := &ast.WhileLoopActionNode{
			Kind:                  ast.LoopFor,
			Body:                  p.parseActionBodyParameter(),
			Variable:              variable,
			VariableRelationships: variableRels,
			Collection:            collection,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	if _, ok := p.expect(lexer.LBrace, "expected '{' for for-loop body"); !ok {
		en := &ast.ErrorNode{Message: "expected '{'"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	// Parse body as mixed content (declarations + behavioral statements)
	var body []ast.Node
	leave := p.pushBodyContext(bodyAction)
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset

		// Try direction parameters
		if p.isDirectionKeyword() {
			body = append(body, p.parseDirectionParameter())
			continue
		}

		// Try declarations
		if p.atDefUsageStart() {
			m := p.parseBodyMember()
			if m != nil {
				body = append(body, m)
			}
			if p.peek().Span.Offset == before {
				p.advance()
			}
			continue
		}

		// Parse behavioral statements
		body = append(body, p.parseActionMember())

		// Ensure progress
		if p.peek().Span.Offset == before {
			p.advance()
		}
	}
	leave()

	p.expect(lexer.RBrace, "expected '}'")

	// A `for` loop has no condition: iteration is driven by the collection, and
	// the variable it binds each element to is a member of the loop's body scope.
	node := &ast.WhileLoopActionNode{
		Kind:                  ast.LoopFor,
		Body:                  body,
		Variable:              variable,
		VariableRelationships: variableRels,
		Collection:            collection,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseDefaultTargetSuccession parses `else <target>;` (SysML.xtext
// DefaultTargetSuccession): the branch of the preceding decision taken when no
// guarded branch is, which an unguarded edge is how the executor routes.
func (p *Parser) parseDefaultTargetSuccession(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	target := p.parseQualifiedName()
	p.expect(lexer.Semicolon, "expected ';' after else branch")

	node := &ast.ControlFlowEdge{
		Source: &ast.QualifiedName{}, // empty source = the member before it
		Target: target,
		IsElse: true,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseIfAction parses: if condition { thenBody } [else { elseBody }]
func (p *Parser) parseIfAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// Parse condition expression
	condition := p.ParseExpression()

	// Check for shorthand guard succession: if <expr> then <target>;
	if p.atKeyword("then") {
		p.advance() // consume 'then'
		target := p.parseQualifiedName()
		p.expect(lexer.Semicolon, "expected ';' after guard succession")

		// Return ControlFlowEdge with implicit source (decision node) and guard
		node := &ast.ControlFlowEdge{
			Source: &ast.QualifiedName{}, // empty source = implicit decision node
			Target: target,
			Guard:  condition,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}

	thenStart := p.peek().Span.Offset
	var thenBranch *ast.IfBranchNode
	switch {
	case p.atKeyword("action"):
		thenBranch = &ast.IfBranchNode{Kind: ast.IfBranchThen, Body: p.parseActionBodyParameter()}
		thenBranch.NodeSpan = p.spanFrom(thenStart)
	case p.at(lexer.LBrace):
		p.advance() // consume '{'
		thenBranch = p.parseIfBranch(ast.IfBranchThen, thenStart, "expected '}' after if body")
	default:
		p.error(p.peek().Span, "expected '{' after if condition")
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
			Message:  "expected '{' after if condition",
		}
	}

	// Check for optional 'else' clause
	var elseBranch *ast.IfBranchNode
	elseStart := p.peek().Span.Offset
	if p.acceptKeyword("else") {
		switch {
		case p.atKeyword("action"):
			elseBranch = &ast.IfBranchNode{Kind: ast.IfBranchElse, Body: p.parseActionBodyParameter()}
			elseBranch.NodeSpan = p.spanFrom(elseStart)
		case p.atKeyword("if"):
			// `else if …` is an if node in the else parameter (SysML.xtext
			// IfNodeParameterMember), so the nested node is the branch's body.
			elseBranch = &ast.IfBranchNode{Kind: ast.IfBranchElse, Body: []ast.Node{p.parseIfAction(p.advance())}}
			elseBranch.NodeSpan = p.spanFrom(elseStart)
		case p.at(lexer.LBrace):
			p.advance() // consume '{'
			elseBranch = p.parseIfBranch(ast.IfBranchElse, elseStart, "expected '}' after else body")
		default:
			p.error(p.peek().Span, "expected '{' after else")
		}
	}

	node := &ast.IfActionNode{
		Condition: condition,
		Then:      thenBranch,
		Else:      elseBranch,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseIfBranch parses the members of one branch body of an if action, with the
// opening '{' already consumed, and consumes the closing '}'. start is the
// offset the branch's span begins at ('{' for the then branch, 'else' for the
// else branch). Both declarations and behavioral statements are accepted, so
// `if <cond> { action x : Type { body }; first x then y; }` parses.
func (p *Parser) parseIfBranch(kind ast.IfBranchKind, start int, closeMsg string) *ast.IfBranchNode {
	var body []ast.Node
	leave := p.pushBodyContext(bodyAction)
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset

		// Direction parameters first.
		if p.isDirectionKeyword() {
			body = append(body, p.parseDirectionParameter())
			continue
		}

		// Declarations (action/part/etc).
		if p.atDefUsageStart() {
			if m := p.parseBodyMember(); m != nil {
				body = append(body, m)
			}
			// No progress: advance to avoid an infinite loop.
			if p.peek().Span.Offset == before {
				p.advance()
			}
			continue
		}

		body = append(body, p.parseActionMember())
	}
	leave()
	p.expect(lexer.RBrace, closeMsg)

	branch := &ast.IfBranchNode{Kind: kind, Body: body}
	branch.NodeSpan = p.spanFrom(start)
	return branch
}

// Phase C1: Calculation and Constraint Bodies

// atCalcStatement reports whether the calculation body continues with a
// behavioural statement rather than a member declaration or a result
// expression. `if` also starts a conditional expression, which is told apart by
// the '?' that follows its condition.
func (p *Parser) atCalcStatement() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	switch p.peek().KeywordID {
	case "assign", "while", "loop", "for", "send", "perform", "terminate":
		return true
	case "if":
		return !p.atConditionalExpression()
	}
	return false
}

// atCalcBodyItem reports whether the cursor begins a behavioural item of a
// calculation body (SysML.xtext CalculationBodyItem carries ActionBodyItem): a
// statement, an action node, or a succession (`first a then b;`, `then b;`, `else b;`).
// A member-attached `then` is the body builder's to take before this is asked.
func (p *Parser) atCalcBodyItem() bool {
	if p.atCalcStatement() || p.atActionNodeMember() {
		return true
	}
	return p.atKeyword("first") || p.atKeyword("then") || p.atKeyword("else")
}

// atActionNodeMember reports whether the cursor begins an action node a calculation or case
// body carries (SysML.xtext:1389 ActionNodeMember, from Calculation/CaseBodyItem).
func (p *Parser) atActionNodeMember() bool {
	if _, ok := p.atActionNodeWord(); ok {
		return true
	}
	if !p.at(lexer.Keyword) {
		return false
	}
	switch p.peek().KeywordID {
	case "fork", "join", "merge", "decide":
		return true
	}
	return false
}

// atConditionalExpression reports whether the `if` at the cursor starts a
// conditional expression (`if c ? a else b`) rather than an if statement, by
// looking for the '?' that ends its condition. A brace group is scanned past
// only while the condition goes on after it, so a body expression
// (`xs->exists{...}`) is read as part of the condition while the block of an if
// statement ends the scan.
func (p *Parser) atConditionalExpression() bool {
	depth := 0
	for i := 1; ; i++ {
		tok := p.peekN(i)
		switch tok.Kind {
		case lexer.EOF:
			return false
		case lexer.LParen, lexer.LBracket, lexer.LBrace:
			depth++
		case lexer.RParen, lexer.RBracket, lexer.RBrace:
			if depth == 0 {
				return false
			}
			depth--
			if depth == 0 && tok.Kind == lexer.RBrace && !continuesCondition(p.peekN(i+1)) {
				return false
			}
		case lexer.Question:
			if depth == 0 {
				return true
			}
		case lexer.Semicolon:
			if depth == 0 {
				return false
			}
		}
	}
}

// continuesCondition reports whether tok can go on the condition of a
// conditional expression once a brace group of it has closed.
func continuesCondition(tok lexer.Token) bool {
	switch tok.Kind {
	case lexer.Question, lexer.Arrow, lexer.Dot, lexer.LBracket,
		lexer.Comma, lexer.RParen, lexer.RBracket:
		return true
	}
	_, isOperator := binaryOpForToken(tok)
	return isOperator
}

// atReturnedUsage reports whether `return` opens a usage (`'return' UsageElement`, SysML.xtext:1961):
// a specialization or multiplicity, or an identification (`<s>`? name?) a specialization, `[`,
// a value, `{` or `;` follows. A value or body alone declares nothing (FeatureDeclaration).
func (p *Parser) atReturnedUsage() bool {
	if p.atFeatureSpecialization() || p.at(lexer.LBracket) {
		return true
	}
	if !p.atName() && !p.at(lexer.Lt) {
		return false
	}
	cp := p.checkpoint()
	p.parseIdentification()
	declared := p.atFeatureSpecialization() || p.at(lexer.LBracket) || p.at(lexer.LBrace) ||
		p.at(lexer.Semicolon) || p.valueOperatorAt(0)
	p.restore(cp)
	p.release()
	return declared
}

// parseResultMember parses a `return` member: the declaration of a result
// parameter (SysML.xtext:1961 ReturnParameterMember), anonymous or named,
// typed or valued. A computed result is the body's trailing expression, written
// without `return`, so an expression after `return` is refused here.
func (p *Parser) parseResultMember() ast.Node {
	start := p.peek().Span.Offset

	// Expect 'return' keyword
	if !p.acceptKeyword("return") {
		p.error(p.peek().Span, "expected 'return' in calculation body")
		en := &ast.ErrorNode{Message: "expected 'return' in calculation body"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance() // ensure progress
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	// Parse optional usage kind keyword (e.g., 'attribute')
	// Default to UsageAttribute if not specified
	usageKind := ast.UsageAttribute
	if p.at(lexer.Keyword) {
		if kind, ok := p.usageKind(p.peek().KeywordID); ok {
			usageKind = kind
			p.advance() // consume kind keyword
		}
	}

	// Parse optional feature modifiers after kind keyword
	mods := p.parseFeatureModifiers()

	// A result parameter is a usage, every part optional:
	// `return [modifiers] <s>? name? : Type[mult] = expr { body }`.
	if p.atReturnedUsage() {
		u := &ast.Usage{
			Kind:        usageKind,
			Direction:   ast.DirOut,
			IsResult:    true,
			IsAbstract:  mods.isAbstract,
			IsReference: mods.isReference,
			IsEnd:       mods.isEnd,
			IsConstant:  mods.isConstant,
			IsComposite: mods.isComposite,
			IsPortion:   mods.isPortion,
			IsDerived:   mods.isDerived,
			IsOrdered:   mods.isOrdered,
			IsNonunique: mods.isNonunique,
		}

		if p.atName() || p.at(lexer.Lt) {
			u.Ident = p.parseIdentification()
		}

		// FeatureSpecializationPart: the typing and the specializations, in the
		// order written (`: T`, `:> engine`, `: T[1] nonunique :>> x`).
		p.parseFeatureSpecializationPart(u)

		// Parse optional value 'default [=] expr', '= expr' or ':= expr'
		p.parseUsageValue(u)

		if p.at(lexer.LBrace) || p.at(lexer.Semicolon) {
			members, hasBody := p.parseGenericBody()
			u.Members = members
			u.HasBody = hasBody
		} else {
			p.error(p.peek().Span, msgExpectedReturnEnd)
		}

		u.NodeSpan = p.spanFrom(start)
		return u
	}

	// A value or body alone declares no result (FeatureDeclaration names, specializes
	// or bounds it); the remainder is read so the members after it are kept.
	if p.valueOperatorAt(0) || p.at(lexer.LBrace) {
		const msg = "expected the result's name, specialization or multiplicity after 'return'"
		p.error(p.peek().Span, msg)
		p.parseUsageValue(&ast.Usage{})
		if p.at(lexer.LBrace) || p.at(lexer.Semicolon) {
			p.parseGenericBody()
		}
		en := &ast.ErrorNode{Message: msg}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	// Anything else after `return` is an expression, which no production admits:
	// the value a calculation computes is its trailing expression. The expression
	// is still consumed, so the members after it are read as written.
	const msg = "'return' declares a result parameter; write a computed result as the " +
		"trailing expression of the body, without 'return'"
	p.error(p.peek().Span, msg)
	p.ParseExpression()
	p.expect(lexer.Semicolon, "expected ';' after return expression")

	en := &ast.ErrorNode{Message: msg}
	en.NodeSpan = p.spanFrom(start)
	return en
}

// parseConstraintBody parses the body of a constraint usage.
// Expects '{' already consumed, returns list of ConstraintMember nodes.
// Syntax: constraint example { assert x > 0; assume y != null; assert not z < 0; }
func (p *Parser) parseConstraintBody() []ast.Node {
	return p.parseConstraintMembers(false)
}

// parseConstraintMembers parses the members of a constraint body through its
// closing brace. A constraint body is a calculation body (SysML.xtext
// CalculationBody), so beside its conditions it carries declarations, statements,
// action nodes and successions; the owning-type rule rejects the nodes later.
// A nested body keeps no documentation and no condition that failed to parse.
func (p *Parser) parseConstraintMembers(nested bool) []ast.Node {
	body := p.newBodyBuilder()

	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset

		// A member-attached `then` sequences the members either side of it.
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}

		var member ast.Node
		switch {
		case p.atKeyword("doc"):
			if doc := p.parseDocumentation(before); !nested {
				member = doc
			}
		case p.atKeyword("assert") || p.atKeyword("assume"):
			member = p.parseConstraintMember()
		case p.atConstraintBodyDeclaration():
			member = p.parseBodyMember()
		case p.atCalcBodyItem():
			member = p.parseActionMember()
		default:
			member = p.parseConstraintMember()
		}
		if c, ok := member.(*ast.ConstraintMember); ok && nested && c.Expression == nil && len(c.Body) == 0 {
			member = nil
		}
		body.add(member)

		// Force progress: a member that consumed nothing would spin the loop.
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}

	p.expect(lexer.RBrace, "expected '}' after constraint body")
	return body.finish()
}

// atConstraintBodyDeclaration reports whether a declaration member of a
// calculation body follows: a `return` member, a definition or usage, a
// relationship where a name would go (`:>> x = v;`) or a metadata usage (`@M { … }`).
func (p *Parser) atConstraintBodyDeclaration() bool {
	return p.atKeyword("return") || p.atDefUsageStart() || p.atRelationshipKeyword() ||
		p.atKindlessFeatureTyping() || p.atNamedCalcMember() || p.atMetadataMember()
}

// atMetadataMember tells a metadata usage (`@M;`, `@M { … }`, `@ m : M about x;`)
// from a classification condition of the implicit subject (`@M`, `@M < x`).
func (p *Parser) atMetadataMember() bool {
	if !p.at(lexer.At) {
		return false
	}
	n := 1
	// An identification (`<sn>`? name?) is only one where a typing keyword follows.
	m := n
	if p.peekN(m).Kind == lexer.Lt {
		if !isNameToken(p.peekN(m+1).Kind) || p.peekN(m+2).Kind != lexer.Gt {
			return false
		}
		m += 3
	}
	if isNameToken(p.peekN(m).Kind) {
		m++
	}
	if t := p.peekN(m); t.Kind == lexer.Colon {
		n = m + 1
	} else if t.Kind == lexer.Keyword && (t.KeywordID == "defined" || t.KeywordID == "typed") {
		if by := p.peekN(m + 1); by.Kind != lexer.Keyword || by.KeywordID != "by" {
			return false
		}
		n = m + 2
	}
	// The metaclass, a qualified name.
	if !isNameToken(p.peekN(n).Kind) {
		return false
	}
	for n++; p.peekN(n).Kind == lexer.ColonColon; n += 2 {
		if !isNameToken(p.peekN(n + 1).Kind) {
			return false
		}
	}
	switch t := p.peekN(n); t.Kind {
	case lexer.LBrace, lexer.Semicolon:
		return true
	case lexer.Keyword:
		return t.KeywordID == "about"
	}
	return false
}

// isNameToken reports whether k spells a name segment.
func isNameToken(k lexer.Kind) bool {
	return k == lexer.Identifier || k == lexer.UnrestrictedName
}

// atConstraintCondition reports whether an `assert`/`assume` condition follows,
// as against a named constraint usage (`assert constraint { … }`).
func (p *Parser) atConstraintCondition() bool {
	if !p.atKeyword("assert") && !p.atKeyword("assume") {
		return false
	}
	n := 1
	if t := p.peekN(n); t.Kind == lexer.Keyword && t.KeywordID == "not" {
		n++
	}
	t := p.peekN(n)
	if t.Kind != lexer.Keyword {
		return true
	}
	// A keyword that starts an expression starts a condition too (atExprStart).
	return exprStartKeywords[t.KeywordID]
}

// exprStartKeywords are the keywords that begin an expression rather than a
// declaration; atExprStart accepts the same set.
var exprStartKeywords = map[string]bool{
	"null":  true,
	"true":  true,
	"false": true,
	"new":   true,
	"if":    true,
	"not":   true,
}

// parseConstraintMember parses one constraint member: a bare condition, an
// asserted reference (`assert c;`) or a nested constraint (`assert constraint { … }`).
func (p *Parser) parseConstraintMember() ast.Node {
	start := p.peek().Span.Offset
	keywordSpan := p.peek().Span

	var isAssert bool
	var isNegated bool
	var keyword string

	// Check for 'assert' or 'assume' keyword
	if p.acceptKeyword("assert") {
		isAssert = true
		keyword = "assert"
	} else if p.acceptKeyword("assume") {
		isAssert = false
		keyword = "assume"
	} else {
		// Bare expression (implicit assert) - common in invariants
		// Example: inv piPrecision { RealFunctions::round(pi * 1E20) == 314159265358979323846.0 }
		isAssert = true // Default to assert for bare expressions
	}

	// `not` negates what a keyword states (SysML.xtext AssertConstraintUsage);
	// a bare condition keeps it as the unary operator of its expression.
	if keyword != "" && p.acceptKeyword("not") {
		isNegated = true
	}

	// A nested constraint states its conditions in a body rather than inline:
	// assert constraint [<name>] { <expr> }
	if node := p.tryParseNestedConstraint(start, isAssert, isNegated, keyword); node != nil {
		return node
	}

	// Parse expression
	exprStart := p.peek().Span.Offset
	expr := p.ParseExpression()

	// Semicolon is optional for constraint expressions (especially in inv bodies)
	semi, hasSemi := p.accept(lexer.Semicolon)

	// A keyword states a reference or a `constraint` declaration; the condition
	// itself is written on its own (SysML.xtext AssertConstraintUsage).
	if keyword != "" && !isConditionReference(expr) {
		p.errorWithFixes(keywordSpan,
			fmt.Sprintf("`%s` states a constraint reference or a `constraint` declaration, "+
				"not a condition: write the condition on its own", keyword),
			p.assertedConditionFixes(keywordSpan, exprStart, semi, hasSemi, isNegated)...)
	}

	node := &ast.ConstraintMember{
		IsAssert:   isAssert,
		Keyword:    keyword,
		IsNegated:  isNegated,
		Expression: expr,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// isConditionReference reports whether expr is the reference an `assert`,
// `assume` or `require` member states (SysML.xtext OwnedReferenceSubsetting).
func isConditionReference(expr ast.Node) bool {
	switch expr.(type) {
	case *ast.QualifiedName, *ast.FeatureReference, *ast.FeatureChainExpr:
		return true
	default:
		return false
	}
}

// assertedConditionFixes rewrites `assert <condition>;` as the bare condition,
// keeping a negation as `not (…)` so the condition means what it did.
func (p *Parser) assertedConditionFixes(keyword source.Span, exprStart int, semi lexer.Token, hasSemi, negated bool) []quickfix.Fix {
	prefix := source.Span{Offset: keyword.Offset, Len: exprStart - keyword.Offset}
	if prefix.Len <= 0 {
		return nil
	}
	if !negated {
		edits := []quickfix.Edit{quickfix.Replace(prefix, "")}
		if hasSemi {
			edits = append(edits, quickfix.Replace(semi.Span, ""))
		}
		return []quickfix.Fix{{Title: "write the condition without `" + p.src.Text(keyword) + "`", Edits: edits, Preferred: true}}
	}
	// Without the semicolon there is nowhere to close the parenthesis.
	if !hasSemi {
		return nil
	}
	return []quickfix.Fix{{
		Title:     "write the condition as `not (…)`",
		Edits:     []quickfix.Edit{quickfix.Replace(prefix, "not ("), quickfix.Replace(semi.Span, ")")},
		Preferred: true,
	}}
}

// requirementConditionFixes rewrites `require <condition>;` as the constraint
// declaration a requirement body admits (SysML.xtext RequirementConstraintUsage).
func (p *Parser) requirementConditionFixes(keyword source.Span, exprStart int, semi lexer.Token, hasSemi bool) []quickfix.Fix {
	prefix := source.Span{Offset: keyword.Offset, Len: exprStart - keyword.Offset}
	if prefix.Len <= 0 || !hasSemi {
		return nil
	}
	word := p.src.Text(keyword)
	return []quickfix.Fix{{
		Title: "state the condition as `" + word + " constraint { … }`",
		Edits: []quickfix.Edit{
			quickfix.Replace(prefix, word+" constraint { "),
			quickfix.Replace(semi.Span, " }"),
		},
		Preferred: true,
	}}
}

// Phase C2: Requirement Bodies

// parseRequirementBody parses the body of a requirement usage.
// Expects '{' already consumed, returns list of requirement members.
// Syntax: requirement example { subject x : Type; assume x > 0; require x.valid; actor a : Actor; }
func (p *Parser) parseRequirementBody() []ast.Node {
	body := p.newBodyBuilder()

	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		// A requirement body carries the members of a definition body
		// (SysML.xtext RequirementBodyItem), a member-attached `then` among them,
		// which takes no body there (DefinitionBodyItem, SysML.xtext:516-524).
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}
		// A member-attached `then` desugars to `succession first a then b;`,
		// which is the form used when writing the converted model back.
		if p.atKeyword("then") {
			body.add(p.parseSuccessionEdge(p.advance(), false))
			continue
		}
		body.add(p.parseRequirementMember())
		// Force progress: a member that consumed nothing would spin the loop.
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}

	p.expect(lexer.RBrace, "expected '}' after requirement body")
	return body.finish()
}

// parseRequirementMember parses one requirement member: subject/assume/require/actor/doc or general body members
func (p *Parser) parseRequirementMember() ast.Node {
	start := p.peek().Span.Offset

	// Check for requirement-specific keywords FIRST (before tryParseDeclaration)
	// These keywords have special meaning in requirement context that differs from general usage
	if node := p.parseKeywordedRequirementMember(start); node != nil {
		return node
	}

	// Try general declaration (nested requirements, features, etc.)
	if node := p.tryParseDeclaration(); node != nil {
		// Validate that tryParseDeclaration didn't just accept garbage
		// Example: "require ;" gets parsed as anonymous constraint usage by tryParseDeclaration
		if usage, ok := node.(*ast.Usage); !ok || usageIsSubstantive(usage) {
			return node // Valid declaration, use it
		}
		// Nothing meaningful: likely a keyword we should handle specially, so
		// fall through to the fallback below.
	}

	// Graceful fallback: parse as general body member (expression, statement, etc.)
	// This handles any legal construct we haven't explicitly enumerated
	node := p.parseBodyMember()

	// Validate fallback didn't just accept garbage
	// If we got an ErrorNode, the parser already diagnosed it
	if _, isError := node.(*ast.ErrorNode); isError {
		return node
	}

	// If fallback produced something suspicious, diagnose it
	// Example: anonymous usage with no relationships (like bare "require ;")
	if usage, ok := node.(*ast.Usage); ok && !usageIsSubstantive(usage) {
		p.error(node.Span(), "expected 'subject', 'assume', 'require', 'actor', or a valid body member")
		return &ast.ErrorNode{Message: "unexpected requirement member"}
	}

	return node
}

// usageIsSubstantive reports whether a usage declares anything: a name, a
// relationship of any kind (`ref concern :>> self : ConcernCheck` takes its name
// from its redefinition), a value, a body or connector/flow ends (an anonymous
// `connection connect a to b`). A usage with none of those came from a keyword
// the requirement body parser handles itself.
func usageIsSubstantive(u *ast.Usage) bool {
	return u.Ident.Name != "" || len(u.Relationships) > 0 || u.Value != nil || len(u.Members) > 0 ||
		len(u.ConnectorEnds) > 0 || u.FlowEnds != nil
}

// parseKeywordedRequirementMember parses a `subject`, `assume` or `require` member, reporting
// prefix metadata written ahead of the keyword and reading it as if it followed; nil if neither.
func (p *Parser) parseKeywordedRequirementMember(start int) ast.Node {
	kw := p.peekN(p.prefixLookahead())
	if kw.Kind != lexer.Keyword {
		return nil
	}
	switch kw.KeywordID {
	case "subject":
		if !p.bodyAdmitsMember("subject") {
			return nil
		}
	case "assume", "require":
	default:
		return nil
	}
	prefixes := p.parsePrefixMetadata()
	if len(prefixes) > 0 {
		p.reportMisplacedPrefixMetadata(prefixes, kw)
	}
	p.advance() // the keyword
	prefixes = append(prefixes, p.parsePrefixMetadata()...)
	switch kw.KeywordID {
	case "subject":
		return p.parseSubjectMember(start, prefixes)
	case "assume":
		return p.parseAssumeMember(start, prefixes)
	}
	return p.parseRequireMember(start, prefixes)
}

// parseSubjectMember parses a subject parameter: `subject [name] [: Type] [mult]
// [specializations] [value] (; | body)`; the keyword and its prefix metadata are consumed.
func (p *Parser) parseSubjectMember(start int, prefixes []*ast.PrefixMetadata) ast.Node {
	// A bare `subject;` declares the subject parameter without naming or typing
	// it, as the OMG viewpoint examples write it.
	if p.at(lexer.Semicolon) {
		p.advance()
		node := &ast.SubjectMember{Prefixes: prefixes}
		node.NodeSpan = p.spanFrom(start)
		return node
	}

	// A subject parameter is a full usage (SysML v2 8.2.2.16): an optional
	// identification (`<short> name`), an optional specialization part, a
	// multiplicity, a value and a body. An anonymous one with just a value
	// (`subject = p;`) binds the inherited subject.
	ident := p.parseIdentification()

	var typeRef *ast.QualifiedName
	if p.at(lexer.Colon) {
		p.advance()
		typeRef = p.parseQualifiedName()
	}

	var mult *ast.Multiplicity
	if p.at(lexer.LBracket) {
		mult = p.parseMultiplicity()
	}

	// A subject may redefine the one it inherits: subject subj : View[1] :>> RequirementCheck::subj;
	rels := p.parseRelationships(true)

	// Value part: `= expr`, `:= expr` or `default [=] expr`.
	valueOp, hasValue := p.acceptValueOperator()
	if ident == (ast.Identification{}) && typeRef == nil && len(rels) == 0 && !hasValue {
		p.error(p.peek().Span, "expected a name, ':', a specialization or a value after 'subject'")
		en := &ast.ErrorNode{Message: "expected a name, ':', a specialization or a value after 'subject'"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	var value ast.Node
	if hasValue {
		value = p.ParseExpression()
	}

	node := &ast.SubjectMember{
		Prefixes:          prefixes,
		Ident:             ident,
		TypeRef:           typeRef,
		Multiplicity:      mult,
		Relationships:     rels,
		BindingExpr:       value,
		ValueOperatorSpan: valueOp.span,
		ValueIsDefault:    valueOp.isDefault,
		ValueIsInitial:    valueOp.isInitial,
	}

	if p.at(lexer.LBrace) {
		p.advance()
		node.HasBody = true
		// A subject's body is a UsageBody, not the RequirementBody around it.
		leave := p.pushBodyContext(bodyOther)
		node.Body = p.parseRequirementBody()
		leave()
	} else {
		p.expect(lexer.Semicolon, "expected ';' or '{' after subject declaration")
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseAssumeMember parses: assume <expr>; the keyword and its prefix metadata are consumed.
func (p *Parser) parseAssumeMember(start int, prefixes []*ast.PrefixMetadata) ast.Node {
	// Check for 'assume [#Meta...] [constraint] [<decl>] (; | { body })' pattern
	if p.atKeyword("constraint") || len(prefixes) > 0 {
		declStart := p.peek().Span.Offset
		p.acceptKeyword("constraint")
		d := p.parseOwnedConstraintDecl("assume constraint")
		node := &ast.AssumeMember{
			Prefixes:          prefixes,
			Ident:             d.ident,
			DeclSpan:          p.spanFrom(declStart),
			Relationships:     d.relationships,
			Multiplicity:      d.multiplicity,
			Value:             d.value,
			ValueOperatorSpan: d.valueOp.span,
			ValueIsDefault:    d.valueOp.isDefault,
			ValueIsInitial:    d.valueOp.isInitial,
			HasBody:           d.hasBody,
			Body:              d.body,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}

	// Reference-subsetting form: assume <QualifiedName> [specializations] (; | { body })
	if cr, ok := p.tryParseConstraintReference(); ok {
		node := &ast.AssumeMember{
			Reference:     cr.ref,
			Relationships: cr.relationships,
			Multiplicity:  cr.multiplicity,
			HasBody:       cr.hasBody,
			Body:          cr.body,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}

	// Otherwise the member states a condition inline, which no production admits.
	expr := p.parseRequirementCondition(start, "assume")

	node := &ast.AssumeMember{
		Expression: expr,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseRequireMember parses: require <expr>; the keyword and its prefix metadata are consumed.
func (p *Parser) parseRequireMember(start int, prefixes []*ast.PrefixMetadata) ast.Node {
	// Check for 'require [#Meta...] [constraint] [<decl>] (; | { body })' pattern
	if p.atKeyword("constraint") || len(prefixes) > 0 {
		declStart := p.peek().Span.Offset
		p.acceptKeyword("constraint")
		d := p.parseOwnedConstraintDecl("require constraint")
		node := &ast.RequireMember{
			Prefixes:          prefixes,
			Ident:             d.ident,
			DeclSpan:          p.spanFrom(declStart),
			Relationships:     d.relationships,
			Multiplicity:      d.multiplicity,
			Value:             d.value,
			ValueOperatorSpan: d.valueOp.span,
			ValueIsDefault:    d.valueOp.isDefault,
			ValueIsInitial:    d.valueOp.isInitial,
			HasBody:           d.hasBody,
			Body:              d.body,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}

	// Reference-subsetting form: require <QualifiedName> [specializations] (; | { body })
	if cr, ok := p.tryParseConstraintReference(); ok {
		node := &ast.RequireMember{
			Reference:     cr.ref,
			Relationships: cr.relationships,
			Multiplicity:  cr.multiplicity,
			HasBody:       cr.hasBody,
			Body:          cr.body,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}

	// Otherwise the member states a condition inline, which no production admits.
	expr := p.parseRequirementCondition(start, "require")

	node := &ast.RequireMember{
		Expression: expr,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseRequirementCondition parses what an `assume`/`require` member states
// where neither a `constraint` declaration nor a qualified reference followed.
// A bare name is the reference form; anything else is a condition stated inline,
// which RequirementConstraintUsage does not admit.
func (p *Parser) parseRequirementCondition(start int, keyword string) ast.Node {
	keywordSpan := source.Span{Offset: start, Len: len(keyword)}
	exprStart := p.peek().Span.Offset
	expr := p.ParseExpression()
	semi, hasSemi := p.accept(lexer.Semicolon)
	if !hasSemi {
		p.expect(lexer.Semicolon, "expected ';' after "+keyword+" expression")
	}
	if !isConditionReference(expr) {
		p.errorWithFixes(keywordSpan,
			fmt.Sprintf("`%s` states a constraint reference or a `constraint` declaration, "+
				"not a condition: state the condition as `%s constraint { … }`", keyword, keyword),
			p.requirementConditionFixes(keywordSpan, exprStart, semi, hasSemi)...)
	}
	return expr
}

// ownedConstraintDecl is the declaration and body of the constraint an
// `assume`/`require constraint …` member owns.
type ownedConstraintDecl struct {
	ident         ast.Identification
	relationships []*ast.Relationship
	multiplicity  *ast.Multiplicity
	value         ast.Node
	valueOp       valueOperator
	body          []ast.Node
	hasBody       bool
}

// parseOwnedConstraintDecl parses `ConstraintUsageDeclaration CalculationBody`
// (SysML.xtext:2015, :2070) with `constraint` already consumed: the declaration
// and the body are both optional, so `assume constraint c1 : C;` and
// `assume constraint { … }` are equally well formed.
func (p *Parser) parseOwnedConstraintDecl(what string) ownedConstraintDecl {
	var d ownedConstraintDecl
	if p.atName() || p.at(lexer.Lt) {
		d.ident = p.parseIdentification()
	}
	d.relationships = p.parseRelationships(true)
	if p.at(lexer.LBracket) {
		d.multiplicity = p.parseMultiplicity()
	}
	if op, ok := p.acceptValueOperator(); ok {
		d.valueOp = op
		d.value = p.ParseExpression()
	}
	if p.at(lexer.LBrace) {
		p.advance() // consume '{'
		d.hasBody = true
		d.body = p.parseNestedConstraintConditions()
		return d
	}
	p.expect(lexer.Semicolon, "expected ';' or '{' after '"+what+"'")
	return d
}

// constraintReference is the reference-subsetting form of a requirement
// constraint member: the requirement referenced, its specializations, its body.
type constraintReference struct {
	ref           *ast.QualifiedName
	relationships []*ast.Relationship
	multiplicity  *ast.Multiplicity
	body          []ast.Node
	hasBody       bool
}

// tryParseConstraintReference parses `OwnedReferenceSubsetting
// FeatureSpecializationPart? CalculationBody` (SysML.xtext
// RequirementConstraintUsage), consuming nothing when the member is not it.
func (p *Parser) tryParseConstraintReference() (constraintReference, bool) {
	if !p.atName() {
		return constraintReference{}, false
	}
	cp := p.checkpoint()
	defer p.release()
	ref := p.parseQualifiedName()
	if ref == nil || len(ref.Parts) == 0 {
		p.restore(cp)
		return constraintReference{}, false
	}
	rels := p.parseRelationships(true)
	// The specialization part carries a multiplicity of its own, which may be
	// followed by further specializations: `require c [0..*] :> d;`.
	var mult *ast.Multiplicity
	if p.at(lexer.LBracket) {
		mult = p.parseMultiplicity()
		rels = append(rels, p.parseRelationships(true)...)
	}
	if p.at(lexer.LBrace) {
		p.advance() // consume '{'
		// A constraint's body is a CalculationBody, not the RequirementBody around it.
		leave := p.pushBodyContext(bodyOther)
		members := p.parseRequirementBody()
		leave()
		return constraintReference{ref: ref, relationships: rels, multiplicity: mult, body: members, hasBody: true}, true
	}
	// A body-less bare name stays a condition expression; only a qualified name or
	// a specialization part can be the reference form.
	if p.at(lexer.Semicolon) && (len(ref.Parts) > 1 || len(rels) > 0 || mult != nil) {
		p.advance() // consume ';'
		return constraintReference{ref: ref, relationships: rels, multiplicity: mult}, true
	}
	p.restore(cp)
	return constraintReference{}, false
}

// tryParseNestedConstraint parses `constraint [<name>] { <conditions> }`, the
// nested-constraint form of a constraint member, and returns nil when the member
// is not that form — `constraint` is a valid feature name, so an expression may
// legitimately start with it.
func (p *Parser) tryParseNestedConstraint(start int, isAssert, isNegated bool, keyword string) ast.Node {
	if !p.atKeyword("constraint") {
		return nil
	}
	cp := p.checkpoint()
	defer p.release()
	p.advance() // consume 'constraint'
	var name string
	if p.at(lexer.Identifier) {
		name = p.src.Text(p.peek().Span)
		p.advance()
	}
	if !p.at(lexer.LBrace) {
		p.restore(cp)
		return nil
	}
	p.advance() // consume '{'
	node := &ast.ConstraintMember{
		IsAssert:  isAssert,
		Keyword:   keyword,
		IsNegated: isNegated,
		Name:      name,
		Body:      p.parseNestedConstraintConditions(),
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseNestedConstraintConditions parses the body of the anonymous constraint an
// `assume`/`require constraint { … }` member owns, through its closing brace,
// and returns its members. Every condition is kept: a constraint body may state
// more than one.
func (p *Parser) parseNestedConstraintConditions() []ast.Node {
	// A constraint's body is a CalculationBody, not the body around it.
	defer p.pushBodyContext(bodyOther)()
	return p.parseConstraintMembers(true)
}

// Phase C4: State Body Parsing

// parseStateBody parses the body of a state usage.
// Expects '{' already consumed, returns list of state members.
func (p *Parser) parseStateBody() []ast.Node {
	body := p.newBodyBuilder()
	allowBody := true

	for !p.at(lexer.RBrace) && !p.atEOF() {
		// A member-attached `then` sequences the members either side of it; a
		// `then` naming states (`succession first idle then done;`) is a member of its own,
		// which parseStateMember reads (see succession.go).
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}
		member := p.parseStateMember(allowBody)
		allowBody = successionBodyAllowed(member, allowBody)
		body.add(member)
	}

	p.expect(lexer.RBrace, "expected '}' after state body")
	return body.finish()
}

// successionBodyAllowed reports whether a `then` after member may carry a body: only a
// TargetTransitionUsage does (SysML.xtext:1764), not an EntryTransitionMember (:1796-1801).
func successionBodyAllowed(member ast.Node, allowed bool) bool {
	switch member.(type) {
	case *ast.EntryMember, *ast.DoMember, *ast.ExitMember:
		return false
	case *ast.SuccessionEdge, *ast.ControlFlowEdge:
		return allowed
	}
	return true
}

// parseStateMember parses one state member: entry/do/exit/state/transition, or general body member.
// allowBody admits a body on a `then`, which only a TargetTransitionUsage takes.
func (p *Parser) parseStateMember(allowBody bool) ast.Node {
	start := p.peek().Span.Offset

	// Handle doc keyword specially (parseDocumentation consumes it)
	if p.atKeyword("doc") {
		return p.parseDocumentation(start)
	}

	// The words of our own state notation are names the lexer does not reserve,
	// so they are matched by the shape around them (see notation.go).
	if w, ok := p.atStateNotationWord(); ok {
		p.advance()
		switch w {
		case "choice":
			return p.parsePseudostate(start, w, ast.PseudostateChoice)
		case "junction":
			return p.parsePseudostate(start, w, ast.PseudostateJunction)
		case "history":
			// Bare `history <name>;` is shallow: SysML v2 has no history notation, so
			// UML's H vs H* is the reference for this OpenSysML extension.
			return p.parsePseudostate(start, w, ast.PseudostateShallowHistory)
		case "shallow", "deep":
			kind := ast.PseudostateShallowHistory
			if w == "deep" {
				kind = ast.PseudostateDeepHistory
			}
			p.advance() // consume 'history'
			return p.parsePseudostate(start, w+" history", kind)
		case "defer":
			return p.parseDeferMember(start)
		}
	}

	// Check for state-specific keywords first
	if p.at(lexer.Keyword) {
		tok := p.peek()
		kw := tok.KeywordID

		switch kw {
		case "entry":
			// `entry point <name>;` declares an entry point pseudostate; `entry ...`
			// anything else is the state's entry action.
			if p.atPointPseudostate() {
				p.advance() // consume 'entry'
				p.advance() // consume 'point'
				return p.parsePseudostate(start, "entry point", ast.PseudostateEntry)
			}
			p.advance()
			return p.parseEntryMember(start)
		case "do":
			p.advance()
			return p.parseDoMember(start)
		case "exit":
			if p.atPointPseudostate() {
				p.advance() // consume 'exit'
				p.advance() // consume 'point'
				return p.parsePseudostate(start, "exit point", ast.PseudostateExit)
			}
			p.advance()
			return p.parseExitMember(start)
		case "state":
			// Check for a simple declaration (`state name;`) or full usage
			// (`state name { ... }`).
			// Lookahead: a name/keyword and semicolon means SubstateMember;
			// otherwise parse a full state usage declaration.
			nextTok := p.peekN(1)
			isNameOrKeyword := nextTok.Kind == lexer.Identifier || nextTok.Kind == lexer.Keyword
			if isNameOrKeyword && p.peekN(2).Kind == lexer.Semicolon {
				p.advance()
				return p.parseSubstateMember(start)
			}
			// Full state usage - parse as body member
			return p.parseBodyMember()
		case "fork":
			p.advance()
			return p.parsePseudostate(start, kw, ast.PseudostateFork)
		case "join":
			p.advance()
			return p.parsePseudostate(start, kw, ast.PseudostateJoin)
		case "transition":
			// A named transition (`transition <name> first …`) is a declaration;
			// an unnamed one heads a state machine transition.
			if p.peekN(1).Kind == lexer.Identifier && p.peekN(2).Kind == lexer.Keyword && p.peekN(2).KeywordID == "first" {
				// Transition usage - parse as general body member (declaration)
				return p.parseBodyMember()
			}
			// State machine transition
			p.advance()
			return p.parseTransitionMember(start)
		case "first":
			// Initial node: first <name> then <target>; a chained end makes it a
			// SuccessionAsUsage instead.
			if p.atChainedFirstSuccession() {
				return p.parseSuccessionAsUsage(start)
			}
			return p.parseInitialNode(p.advance())
		case "then":
			// Standalone implicit-source succession and inline statement forms.
			return p.parseSuccessionEdge(p.advance(), allowBody)
		case "accept", "if":
			// A target transition stated by its trigger or guard alone:
			// `accept <signal> then <state>;`, `if <guard> then <state>;`.
			return p.parseAcceptTransition(start)
		}
	}

	// A member-leading succession is not a SysML succession production.
	if p.at(lexer.Identifier) && p.peekN(1).Kind == lexer.Keyword && p.peekN(1).KeywordID == "then" {
		msg := "a succession names both ends as `first <source> then <target>`"
		p.error(p.peek().Span, msg)
		for !p.at(lexer.Semicolon) && !p.atEOF() {
			p.advance()
		}
		p.expect(lexer.Semicolon, "expected ';' after succession")
		en := &ast.ErrorNode{Message: msg}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	// Not a state-specific keyword - try parsing as general body member
	// This allows succession, binding, feature declarations, etc. in state bodies
	return p.parseBodyMember()
}

// atAcceptNode reports whether the parser is at an accept node declaration
// (SysML.xtext `AcceptNode`): the `accept` keyword, optionally preceded by the
// `action` keyword and the node's own name.
func (p *Parser) atAcceptNode() bool {
	if p.atKeyword("accept") {
		return true
	}
	if !p.atKeyword("action") {
		return false
	}
	if p.peekN(1).Kind == lexer.Keyword && p.peekN(1).KeywordID == "accept" {
		return true
	}
	switch p.peekN(1).Kind {
	case lexer.Identifier, lexer.UnrestrictedName, lexer.Lt:
	default:
		return false
	}
	// `action <name> accept …`, and `action <shortName> name accept …`, whose
	// identification spends four tokens before the keyword.
	for i := 1; i <= 5; i++ {
		tok := p.peekN(i)
		if tok.Kind == lexer.Keyword {
			return tok.KeywordID == "accept"
		}
		switch tok.Kind {
		case lexer.Identifier, lexer.UnrestrictedName, lexer.Lt, lexer.Gt:
			continue
		default:
			return false
		}
	}
	return false
}

// parseAcceptNode parses an accept node: an action that waits for the occurrence
// its payload parameter describes, optionally at a port (`via`), and may carry a
// body like any other action node.
//
//	('action' <name>?)? accept <payload> ('via' <port>)? (';' | '{' … '}')
func (p *Parser) parseAcceptNode(start int, vis ast.Visibility, trivia []ast.Trivia) ast.Node {
	var ident ast.Identification
	// `action accept …` names no node of its own, so the keyword must not be read
	// as the declaration's name.
	if p.acceptKeyword("action") && !p.atKeyword("accept") {
		ident = p.parseIdentification()
	}
	p.advance() // consume 'accept'

	action := &ast.Usage{
		Kind:    ast.UsageAction,
		Keyword: "action",
		Ident:   ident,
	}

	param := p.parsePayloadParameter()
	member := &ast.Membership{Member: param}
	member.NodeSpan = param.Span()
	action.Members = append(action.Members, member)

	// `via <port>`: the port the occurrence must arrive at. It relates the accept
	// to a port rather than specializing it, so it is its own relationship kind.
	if p.acceptKeyword("via") {
		portStart := p.peek().Span.Offset
		rel := &ast.Relationship{Kind: ast.RelVia, Target: p.parseChainedName()}
		rel.NodeSpan = p.spanFrom(portStart)
		action.Relationships = append(action.Relationships, rel)
	}

	if p.at(lexer.LBrace) {
		p.advance()
		leave := p.pushBodyContext(bodyAction)
		action.Members = append(action.Members, p.parseActionBodyMixed()...)
		leave()
		action.HasBody = true
	} else if !p.atKeyword("then") {
		p.expect(lexer.Semicolon, "expected ';' after accept action")
	}

	action.NodeSpan = p.spanFrom(start)
	action.SetLeadingTrivia(trivia)

	m := &ast.Membership{Visibility: vis, Member: action}
	m.NodeSpan = action.Span()
	m.SetLeadingTrivia(trivia)
	return m
}

// atPayloadSpecialization reports whether the parser is at a payload parameter
// declared with a specialization part: `msg : Data`, `:> shutDown`,
// `p :>> Base::event` (SysML.xtext `PayloadFeatureSpecializationPart`). A bare
// name is not one — it types the payload and is read as a signal reference.
func (p *Parser) atPayloadSpecialization() bool {
	i := 0
	if p.atNameOrKeyword() {
		i = 1
	}
	switch p.peekN(i).Kind {
	case lexer.Colon, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt:
		return true
	}
	return false
}

// parsePayloadParameter parses the payload parameter of an accept — the feature
// the accepted occurrence binds to (SysML.xtext `PayloadParameter`). Its
// specialization part says what is accepted: a typing (`: Data`) accepts
// occurrences of a type, a subsetting (`:> shutDown`) occurrences of that event
// feature. A trigger value (`at`/`after`/`when`) is kept as the parameter's
// value, which is what `TriggerValuePart` declares it to be.
func (p *Parser) parsePayloadParameter() *ast.Usage {
	start := p.peek().Span.Offset
	param := &ast.Usage{
		Kind:        ast.UsageAttribute,
		IsReference: true,
		// The payload is what the accept yields to whatever follows it, so the
		// parameter is an output (SysML v2 §8.3.17: AcceptActionUsage's payload
		// parameter).
		Direction: ast.DirOut,
		IsAccept:  true,
	}

	switch {
	case p.namesPayloadType():
		// `accept Data`, `accept ISQ::Time`: the one name types the payload
		// rather than naming it (SysML.xtext `Payload` third alternative, an
		// OwnedFeatureTyping).
		typeStart := p.peek().Span.Offset
		rel := &ast.Relationship{Kind: ast.RelTyping, Target: p.parseQualifiedName()}
		rel.NodeSpan = p.spanFrom(typeStart)
		param.Relationships = append(param.Relationships, rel)
	default:
		if p.atName() || p.at(lexer.Lt) {
			param.Ident = p.parseIdentification()
		}
		if p.atPayloadOperator() {
			rels := p.parseRelationships(true)
			param.Relationships = append(param.Relationships, rels...)
		}
		if p.atTriggerKeyword() {
			param.Value = p.parseTriggerExpression()
		}
	}

	if len(param.Relationships) == 0 && param.Value == nil && param.Ident.Name == "" {
		p.error(p.peek().Span, "expected the payload of the accept: a type (`accept Warning`), a named parameter (`accept w : Warning`), an event (`accept :> shutDown`) or a trigger (`accept when x > 1`)")
	}

	param.NodeSpan = p.spanFrom(start)
	return param
}

// namesPayloadType reports whether the payload is written as a bare name, which
// types it rather than naming it.
func (p *Parser) namesPayloadType() bool {
	if !p.atName() {
		return false
	}
	for i := 1; ; i += 2 {
		if p.peekN(i).Kind != lexer.ColonColon {
			// A name followed by anything but `::` ends the payload only when no
			// specialization, trigger or name follows it.
			switch tok := p.peekN(i); tok.Kind {
			case lexer.Colon, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt,
				lexer.Identifier, lexer.UnrestrictedName:
				return false
			case lexer.Keyword:
				return !isTriggerKeyword(tok.KeywordID)
			}
			return true
		}
		if k := p.peekN(i + 1).Kind; k != lexer.Identifier && k != lexer.UnrestrictedName {
			return false
		}
	}
}

// atTriggerKeyword reports whether the parser is at the keyword of a trigger
// expression.
func (p *Parser) atTriggerKeyword() bool {
	tok := p.peek()
	return tok.Kind == lexer.Keyword && isTriggerKeyword(tok.KeywordID)
}

// isTriggerKeyword reports whether a keyword introduces a trigger expression
// (SysML.xtext `TimeTriggerKind` and `ChangeTriggerKind`).
func isTriggerKeyword(kw string) bool {
	switch kw {
	case "at", "after", "when":
		return true
	}
	return false
}

// atPayloadOperator reports whether the parser is at a specialization operator,
// which a payload parameter may begin with when it declares no name.
func (p *Parser) atPayloadOperator() bool {
	switch p.peek().Kind {
	case lexer.Colon, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt:
		return true
	}
	return false
}

// parseTriggerExpression parses the trigger a payload parameter takes its value
// from (SysML.xtext `TriggerExpression`): `at <instant>` and `after <duration>`
// give a time event, `when <condition>` a change event.
func (p *Parser) parseTriggerExpression() ast.Node {
	kwStart := p.peek().Span.Offset
	if p.atKeyword("when") {
		p.advance()
		evt := &ast.ChangeEvent{Condition: p.ParseExpression()}
		evt.NodeSpan = p.spanFrom(kwStart)
		return evt
	}
	absolute := p.atKeyword("at")
	p.advance() // consume 'at' / 'after'
	evt := &ast.TimeEvent{Duration: p.ParseExpression(), Absolute: absolute}
	evt.NodeSpan = p.spanFrom(kwStart)
	return evt
}

// parseTriggerEvent parses the event of a transition trigger, the part after
// `accept`: a time event (`at <instant>` / `after <duration>`), a change event
// (`when <condition>`), a call event (`<operation>(<params>)`), a payload
// parameter (`<name> : <Type>`, `:> <event>`) or a bare signal name. The event
// kind is decided here so lowering never has to re-derive it.
func (p *Parser) parseTriggerEvent() ast.Node {
	if p.atKeyword("at") || p.atKeyword("after") || p.atKeyword("when") {
		return p.parseTriggerExpression()
	}

	// A payload parameter declared with a specialization: `accept msg : Data`,
	// `accept :> shutDown`, `accept p :> shutDown` (SysML.xtext
	// `PayloadParameter` → `Payload` → `PayloadFeatureSpecializationPart`).
	if p.atPayloadSpecialization() {
		return p.parsePayloadParameter()
	}

	// Bare name: a signal reference, or a call event when an argument list follows.
	nameStart := p.peek().Span.Offset
	name := p.parseQualifiedNameRelaxed()
	if name == nil {
		// An unnamed trigger is an error node, not a nil trigger: a nil one would
		// read as a completion transition and fire on its own.
		en := &ast.ErrorNode{Message: "expected an event after 'accept'"}
		en.NodeSpan = p.spanFrom(nameStart)
		return en
	}
	if p.at(lexer.LParen) {
		return p.parseCallEvent(nameStart, name)
	}
	return name
}

// parseCallEvent parses the argument list of a call trigger, `(<name>, ...)`,
// after its operation name. The names are matched against the arguments of the
// invocation, so `accept setSpeed(value)` fires only for a call carrying a
// `value` argument, while `accept setSpeed()` fires for any call to it.
func (p *Parser) parseCallEvent(start int, operation *ast.QualifiedName) ast.Node {
	p.advance() // consume '('

	var params []ast.NameSegment
	for !p.at(lexer.RParen) && !p.atEOF() {
		seg, ok := p.parseNameSegmentRelaxed()
		if !ok {
			p.error(p.peek().Span, "expected parameter name in call trigger")
			en := &ast.ErrorNode{Message: "expected parameter name in call trigger"}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
		params = append(params, seg)
		if !p.accept2(lexer.Comma) {
			break
		}
	}

	if !p.accept2(lexer.RParen) {
		p.error(p.peek().Span, "expected ')' after call trigger parameters")
		en := &ast.ErrorNode{Message: "expected ')' after call trigger parameters"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	evt := &ast.CallEvent{Operation: operation, Parameters: params}
	evt.NodeSpan = p.spanFrom(start)
	return evt
}

// parseAcceptTransition parses a transition stated without a source, which is
// the state containing it (SysML.xtext `TargetTransitionUsage`):
//
//	accept <trigger> [via <port>] [if <guard>] [do <effect>] then <target>;
//	if <guard> [do <effect>] then <target>;
func (p *Parser) parseAcceptTransition(start int) ast.Node {
	return p.parseTransitionTail(start, ast.NameSegment{}, nil)
}

// stateSubactionKind names which of a state's subactions is being parsed: the
// StateSubactionKind of SysML.xtext (/* STATES */).
type stateSubactionKind string

const (
	subactionEntry stateSubactionKind = "entry"
	subactionDo    stateSubactionKind = "do"
	subactionExit  stateSubactionKind = "exit"
)

// isStateSubactionKeyword reports whether kw opens a StateSubactionMember.
func isStateSubactionKeyword(kw string) bool {
	switch stateSubactionKind(kw) {
	case subactionEntry, subactionDo, subactionExit:
		return true
	}
	return false
}

// parseEntryMember parses a state's entry subaction; the keyword is consumed.
func (p *Parser) parseEntryMember(start int) ast.Node {
	return p.parseStateSubaction(start, subactionEntry)
}

// parseDoMember parses a state's do subaction; the keyword is consumed.
func (p *Parser) parseDoMember(start int) ast.Node {
	return p.parseStateSubaction(start, subactionDo)
}

// parseExitMember parses a state's exit subaction; the keyword is consumed.
func (p *Parser) parseExitMember(start int) ast.Node {
	return p.parseStateSubaction(start, subactionExit)
}

// parseStateSubaction parses the action of an entry/do/exit subaction, whose
// keyword is already consumed:
//
//	StateActionUsage : EmptyActionUsage ';' | PerformedActionUsage ActionBody
//
// (SysML.xtext, /* STATES */). A performed action is either an inline action
// usage (`entry action warmUp;`), a behavioral statement (`entry assign x := 1;`)
// or a reference to an action declared elsewhere (`entry warmUp;`), the last
// being the reference-subsetting form of PerformActionUsageDeclaration. The
// braced form (`entry { ... }`) is an OpenSysML extension over that grammar.
func (p *Parser) parseStateSubaction(start int, kind stateSubactionKind) ast.Node {
	actions, err := p.parseStateSubactionActions(start, kind)
	if err != nil {
		return err
	}
	var node ast.Node
	switch kind {
	case subactionEntry:
		node = &ast.EntryMember{NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)}, Actions: actions}
	case subactionDo:
		node = &ast.DoMember{NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)}, Actions: actions}
	case subactionExit:
		node = &ast.ExitMember{NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)}, Actions: actions}
	}
	return node
}

// parseStateSubactionActions parses the action itself, returning an ErrorNode
// instead when the subaction is followed by something that starts no action.
func (p *Parser) parseStateSubactionActions(start int, kind stateSubactionKind) ([]ast.Node, ast.Node) {
	// EmptyActionUsage ';'
	if p.at(lexer.Semicolon) {
		p.advance()
		return nil, nil
	}

	// `<kind> { ... }`, and the `entry do { ... }` spelling of it.
	if p.at(lexer.LBrace) {
		return p.parseStateSubactionBlock(kind), nil
	}
	if kind != subactionDo && p.atKeyword("do") && p.peekN(1).Kind == lexer.LBrace {
		p.advance() // consume 'do'
		return p.parseStateSubactionBlock(kind), nil
	}

	// An inline action usage or definition: `<kind> action warmUp : WarmUp;`.
	if p.atKeyword("action") {
		member := p.parseBodyMember()
		markStateSubaction(member, kind)
		return []ast.Node{member}, nil
	}

	// A behavioral statement: `<kind> assign x := 1;`, `<kind> send s to t;`.
	if p.isBehavioralKeyword() {
		return []ast.Node{p.parseActionMember()}, nil
	}

	// PerformActionUsageDeclaration by reference: `<kind> warmUp;`.
	if p.atName() {
		return []ast.Node{p.parsePerformedActionReference(p.peek().Span.Offset, featureMods{}, string(kind))}, nil
	}

	msg := fmt.Sprintf("expected an action, an action reference or '{' after '%s'", kind)
	p.error(p.peek().Span, msg)
	en := &ast.ErrorNode{Message: msg}
	en.NodeSpan = p.spanFrom(start)
	return nil, en
}

// markStateSubaction records the subaction keyword as the prefix of the action
// usage it introduces: `entry action ::> a` performs `a` just as `perform
// action ::> a` does (SysML.xtext StateActionUsage → PerformActionUsage).
func markStateSubaction(member ast.Node, kind stateSubactionKind) {
	m, ok := member.(*ast.Membership)
	if !ok {
		return
	}
	if u, ok := m.Member.(*ast.Usage); ok && u.Kind == ast.UsageAction && u.PrefixKeyword == "" {
		u.PrefixKeyword = string(kind)
	}
}

// parseStateSubactionBlock parses the braced action sequence of a subaction;
// the '{' is at the cursor.
func (p *Parser) parseStateSubactionBlock(kind stateSubactionKind) []ast.Node {
	p.advance() // consume '{'
	defer p.pushBodyContext(bodyAction)()
	var actions []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		actions = append(actions, p.parseActionMember())
	}
	p.expect(lexer.RBrace, fmt.Sprintf("expected '}' after %s actions", kind))
	return actions
}

// parseSubstateMember parses: state <name>;
func (p *Parser) parseSubstateMember(start int) ast.Node {
	// 'state' already consumed

	// Expect identifier or keyword (for state names like 'off', 'on', 'normal')
	if !p.atNameOrKeyword() {
		p.error(p.peek().Span, "expected identifier or keyword after 'state'")
		en := &ast.ErrorNode{Message: "expected identifier after 'state'"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	nameToken := p.peek()
	name := p.src.Text(nameToken.Span)
	p.advance()

	p.expect(lexer.Semicolon, "expected ';' after state name")

	node := &ast.SubstateMember{
		Name:     name,
		NameSpan: nameToken.Span,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// atPointPseudostate reports whether the `entry`/`exit` keyword at the cursor
// starts an entry/exit point pseudostate — `entry point <name>;` — rather than
// an entry/exit action. `point` is matched contextually rather than reserved as
// a keyword, because models routinely name features `point`.
func (p *Parser) atPointPseudostate() bool {
	point := p.peekN(1)
	if point.Kind != lexer.Identifier || p.src.Text(point.Span) != "point" {
		return false
	}
	name := p.peekN(2)
	if name.Kind != lexer.Identifier && name.Kind != lexer.Keyword {
		return false
	}
	return p.peekN(3).Kind == lexer.Semicolon
}

// parseDeferMember parses `defer <event> [, <event>]* ;` in a state body: the
// events the state retains while it is active instead of dropping them. Each
// event is parsed exactly like a transition trigger, so a signal name and a
// call event (`defer setSpeed(value)`) both work.
func (p *Parser) parseDeferMember(start int) ast.Node {
	// 'defer' already consumed
	if p.at(lexer.Semicolon) || p.atEOF() || p.at(lexer.RBrace) {
		p.error(p.peek().Span, "expected an event after 'defer'")
		en := &ast.ErrorNode{Message: "expected an event after 'defer'"}
		if p.at(lexer.Semicolon) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	var triggers []ast.Node
	for {
		triggers = append(triggers, p.parseTriggerEvent())
		if !p.accept2(lexer.Comma) {
			break
		}
	}

	p.expect(lexer.Semicolon, "expected ';' after deferred events")

	node := &ast.DeferMember{Triggers: triggers}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parsePseudostate parses a pseudostate declaration in a state body:
// choice/junction/fork/join <name>;. The keyword is already consumed.
func (p *Parser) parsePseudostate(start int, keyword string, kind ast.PseudostateKind) ast.Node {
	if !p.at(lexer.Identifier) && !p.at(lexer.Keyword) {
		p.error(p.peek().Span, fmt.Sprintf("expected name after '%s'", keyword))
		en := &ast.ErrorNode{Message: fmt.Sprintf("expected %s name", keyword)}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	name := p.src.Text(p.peek().Span)
	p.advance()
	p.expect(lexer.Semicolon, fmt.Sprintf("expected ';' after %s name", keyword))

	ps := &ast.PseudostateNode{
		Kind:    kind,
		Name:    name,
		Keyword: keyword,
	}
	ps.NodeSpan = p.spanFrom(start)
	return ps
}

// parseTransitionMember parses a transition of a state machine, with the
// `transition` keyword already consumed (SysML.xtext `TransitionUsage`):
//
//	transition [<name>] [first] <source> [accept <trigger> [via <port>]]
//	    [if <guard>] [do <effect>] then <target>;
func (p *Parser) parseTransitionMember(start int) ast.Node {
	var name ast.NameSegment
	var source *ast.QualifiedName

	// A name of the transition's own, stated before the `first` marking the source.
	if p.atName() && p.peekN(1).Kind == lexer.Keyword && p.peekN(1).KeywordID == "first" {
		if seg, ok := p.parseNameSegment(); ok {
			name = seg
		}
	}

	switch {
	case p.atKeyword("first"):
		p.advance() // consume 'first'
		source = p.parseChainedName()
	case p.atTransitionClause():
		// `transition [accept …] [if …] [do …] then <target>;` names no source:
		// a TargetTransitionUsage whose source is the enclosing state.
	case p.atName() || p.at(lexer.Keyword):
		// `first` is optional in `TransitionUsage`, so a bare source may be followed
		// straight by its clauses: `transition idle then off;`.
		source = p.parseChainedName()
		if !p.atTransitionClause() {
			msg := "a transition states its ends as `transition first <source> … then <target>;`"
			p.error(p.peek().Span, msg)
			for !p.at(lexer.Semicolon) && !p.at(lexer.RBrace) && !p.atEOF() {
				p.advance()
			}
			p.accept(lexer.Semicolon)
			en := &ast.ErrorNode{Message: msg}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
	}

	return p.parseTransitionTail(start, name, source)
}

// atTransitionClause reports whether the parser is at a clause a transition
// carries after its source: its trigger, guard, effect or target.
func (p *Parser) atTransitionClause() bool {
	return p.atKeyword("accept") || p.atKeyword("when") || p.atKeyword("if") ||
		p.atKeyword("do") || p.atKeyword("then")
}

// parseTransitionTail parses the clauses a transition carries after its source —
// trigger, guard, effect and the `then` naming its target — and the terminating
// ';'. The clauses are read in the order they were written so a misordered
// transition is reported once, at the clause that is out of place, rather than
// silently dropped.
func (p *Parser) parseTransitionTail(start int, name ast.NameSegment, source *ast.QualifiedName) ast.Node {
	node := &ast.TransitionMember{
		Name:     name.Text,
		NameSpan: name.Span,
		Source:   source,
	}

	for {
		switch {
		case p.atKeyword("accept"):
			if node.Trigger != nil {
				p.error(p.peek().Span, "a transition accepts one trigger: write a second transition for the other event")
			}
			acceptStart := p.peek().Span.Offset
			p.advance() // consume 'accept'
			node.Trigger = p.parseTriggerEvent()
			// `via <port>` belongs to the accept, naming the port the accepted
			// occurrence must arrive at (SysML.xtext `AcceptParameterPart`).
			if p.acceptKeyword("via") {
				node.Via = p.parseChainedName()
			}
			node.TriggerSpan = p.spanFrom(acceptStart)
			continue
		case p.atKeyword("when"):
			// `when <event>`: the trigger spelling OpenSysML accepts alongside the
			// standard `accept`. What follows is read as an expression and
			// classified when lowered, so a name states a signal and a condition a
			// change, as it did before the standard spelling was added.
			if node.Trigger != nil {
				p.error(p.peek().Span, "a transition accepts one trigger: write a second transition for the other event")
			}
			whenStart := p.peek().Span.Offset
			p.advance() // consume 'when'
			node.Trigger = p.ParseExpression()
			node.TriggerSpan = p.spanFrom(whenStart)
			continue
		case p.atKeyword("if"):
			p.advance() // consume 'if'
			node.Guard = p.ParseExpression()
			continue
		case p.atKeyword("do"):
			p.advance() // consume 'do'
			effect, err := p.parseTransitionEffect(start)
			if err != nil {
				return err
			}
			node.Effect = effect
			node.HasEffect = true
			continue
		case p.atKeyword("then"):
			p.advance() // consume 'then'
			if node.Target != nil {
				p.error(p.peek().Span, "a transition has one target: the name after 'then'")
			}
			node.Target = p.parseChainedName()
			continue
		}
		break
	}

	if node.Target == nil {
		// A transition without a target names no edge, so it is an error node
		// rather than a member the later tiers would read a missing target from.
		msg := "expected the target of the transition after 'then'"
		p.error(p.peek().Span, msg)
		p.accept(lexer.Semicolon)
		en := &ast.ErrorNode{Message: msg}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	// Both TransitionUsage and TargetTransitionUsage end in ActionBody, so the
	// transition may carry a body instead of ending at ';'.
	node.Members, node.HasBody = p.parseNodeBody(start, "transition")
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseTransitionEffect parses the effect of a transition, whose `do` is already
// consumed: a single action (`do action alarm send Alert() to op`, `do assign
// x := 1`) as SysML.xtext `TransitionUsage` states it, or a braced sequence,
// which OpenSysML also accepts.
func (p *Parser) parseTransitionEffect(start int) ([]ast.Node, ast.Node) {
	if p.at(lexer.LBrace) {
		p.advance() // consume '{'
		defer p.pushBodyContext(bodyAction)()
		var effect []ast.Node
		for !p.at(lexer.RBrace) && !p.atEOF() {
			effect = append(effect, p.parseActionMember())
		}
		p.expect(lexer.RBrace, "expected '}' after effect actions")
		return effect, nil
	}
	if p.atKeyword("action") || p.atKeyword("perform") {
		leave := p.enterTransitionEffect()
		defer leave()
		return []ast.Node{p.parseBodyMember()}, nil
	}
	if p.isBehavioralKeyword() {
		leave := p.enterTransitionEffect()
		defer leave()
		return []ast.Node{p.parseActionMember()}, nil
	}
	msg := "expected an action after 'do': an action declaration (`do action alarm send Alert() to operator`), a behavioral statement or '{'"
	p.error(p.peek().Span, msg)
	en := &ast.ErrorNode{Message: msg}
	en.NodeSpan = p.spanFrom(start)
	return nil, en
}

// parseSendStatement parses: send [<message>] [to <receiver> | via <port>]
// ending in ';' or a body.
func (p *Parser) parseSendStatement(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// The message is an optional argument (SysML.xtext SendNode:
	// `ArgumentValue?`): a send declaring none carries the payload its body
	// redefines instead (`send { in :>> payload = s; }`).
	var message ast.Node
	if !p.at(lexer.LBrace) && !p.at(lexer.Semicolon) && !p.atKeyword("to") && !p.atKeyword("via") {
		message = p.parseNodeArgument()
	}

	// A `to` or `via` clause is optional too, and which one was written decides
	// how the target is interpreted: a receiver, or a port to route through.
	isVia := false
	var target, receiver ast.Node
	switch {
	case p.acceptKeyword("via"):
		isVia = true
		target = p.parseNodeArgument()
		// A `via` may name the receiver too (SysML.xtext SenderReceiverPart):
		// the message is routed out of the port and addressed to it.
		if p.acceptKeyword("to") {
			receiver = p.parseNodeArgument()
		}
	case p.acceptKeyword("to"):
		target = p.parseNodeArgument()
	case !p.at(lexer.LBrace) && !p.at(lexer.Semicolon):
		p.error(p.peek().Span, "expected 'to' or 'via' after send message")
	}

	members, hasBody := p.parseNodeBody(start, "send statement")

	node := &ast.SendStatement{
		Message:  message,
		Target:   target,
		IsVia:    isVia,
		Receiver: receiver,
		Members:  members,
		HasBody:  hasBody,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseNodeArgument parses an argument of an action node — the message a send
// carries, or the receiver or port it names. A name path followed by '{' names
// the argument and opens the node's body, so the brace is left to the body.
func (p *Parser) parseNodeArgument() ast.Node {
	if p.atNamePathBeforeBody() {
		if qn := p.parseChainedName(); qn != nil {
			return qn
		}
	}
	return p.ParseExpression()
}

// atNamePathBeforeBody reports whether the tokens ahead are a name path
// followed by '{' (`send to counter { in :>> payload = s; }`).
func (p *Parser) atNamePathBeforeBody() bool {
	if !p.atName() {
		return false
	}
	for i := 1; ; {
		switch p.peekN(i).Kind {
		case lexer.Dot, lexer.ColonColon:
			switch p.peekN(i + 1).Kind {
			case lexer.Identifier, lexer.UnrestrictedName, lexer.Keyword:
				i += 2
			default:
				return false
			}
		case lexer.LBrace:
			return true
		default:
			return false
		}
	}
}

// parseTerminateStatement parses: terminate [<target>];
func (p *Parser) parseTerminateStatement(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// The occurrence to terminate is optional (SysML.xtext TerminateActionUsage:
	// a `terminate` with no parameter terminates the performing occurrence).
	var target ast.Node
	if !p.at(lexer.Semicolon) && !p.at(lexer.RBrace) &&
		!p.atKeyword("then") && !p.atKeyword("if") && !p.atKeyword("do") {
		target = p.ParseExpression()
	}

	p.expectStatementEnd(start, "expected ';' after terminate statement")

	node := &ast.TerminateStatement{
		Target: target,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}
