package parser

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
)

// ParseExpression parses a full expression (conditional at the lowest level).
func (p *Parser) ParseExpression() ast.Node {
	return p.parseConditional()
}

// parseConditional parses `if cond ? then else else` or falls through.
func (p *Parser) parseConditional() ast.Node {
	if p.atKeyword("if") {
		start := p.peek().Span.Offset
		p.advance() // if
		cond := p.parseBinary(precNullCoalesce)
		p.expect(lexer.Question, "expected '?' in conditional")
		thn := p.parseConditional()
		p.expect2Keyword("else")
		els := p.parseConditional()
		e := &ast.OperatorExpr{Operator: ast.OpConditional, Operands: []ast.Node{cond, thn, els}}
		e.NodeSpan = p.spanFrom(start)
		return e
	}
	return p.parseBinary(precNullCoalesce)
}

// Precedence levels (higher binds tighter). Conditional is handled separately.
const (
	precNullCoalesce   = iota + 1 // ??
	precImplies                   // implies
	precOr                        // | or
	precXor                       // xor
	precAnd                       // & and
	precEquality                  // == != === !==
	precClassify                  // hastype istype @ @@ as meta
	precRelational                // < > <= >=
	precRange                     // ..
	precAdditive                  // + -
	precMultiplicative            // * / %
	precExponent                  // ** ^  (right-assoc)
	precUnary                     // prefix + - ~ not
	precExtent                    // all
)

type binOp struct {
	op         ast.OperatorKind
	prec       int
	rightAssoc bool
	classify   bool // RHS is a type reference, stored in TypeRef
}

// binaryOpFor returns the binary operator for the current token, if any.
func (p *Parser) binaryOpFor() (binOp, bool) {
	return binaryOpForToken(p.peek())
}

// binaryOpForToken returns the binary operator t spells, if any.
func binaryOpForToken(t lexer.Token) (binOp, bool) {
	switch t.Kind {
	case lexer.QuestionQ:
		return binOp{ast.OpNullCoalesce, precNullCoalesce, false, false}, true
	case lexer.Pipe:
		return binOp{ast.OpOr, precOr, false, false}, true
	case lexer.Amp:
		return binOp{ast.OpAnd, precAnd, false, false}, true
	case lexer.EqEq:
		return binOp{ast.OpEq, precEquality, false, false}, true
	case lexer.NotEq:
		return binOp{ast.OpNeq, precEquality, false, false}, true
	case lexer.EqEqEq:
		return binOp{ast.OpEqEqEq, precEquality, false, false}, true
	case lexer.NotEqEq:
		return binOp{ast.OpNeqEqEq, precEquality, false, false}, true
	case lexer.At:
		return binOp{ast.OpAt, precClassify, false, true}, true
	case lexer.AtAt:
		return binOp{ast.OpMetaAt, precClassify, false, true}, true
	case lexer.Lt:
		return binOp{ast.OpLt, precRelational, false, false}, true
	case lexer.Gt:
		return binOp{ast.OpGt, precRelational, false, false}, true
	case lexer.Le:
		return binOp{ast.OpLe, precRelational, false, false}, true
	case lexer.Ge:
		return binOp{ast.OpGe, precRelational, false, false}, true
	case lexer.DotDot:
		return binOp{ast.OpRange, precRange, false, false}, true
	case lexer.Plus:
		return binOp{ast.OpAdd, precAdditive, false, false}, true
	case lexer.Minus:
		return binOp{ast.OpSub, precAdditive, false, false}, true
	case lexer.Star:
		return binOp{ast.OpMul, precMultiplicative, false, false}, true
	case lexer.Slash:
		return binOp{ast.OpDiv, precMultiplicative, false, false}, true
	case lexer.Percent:
		return binOp{ast.OpMod, precMultiplicative, false, false}, true
	case lexer.StarStar:
		return binOp{ast.OpPow, precExponent, true, false}, true
	case lexer.Caret:
		return binOp{ast.OpPow, precExponent, true, false}, true
	case lexer.Keyword:
		switch t.KeywordID {
		case "implies":
			return binOp{ast.OpImplies, precImplies, false, false}, true
		case "or":
			return binOp{ast.OpConditionalOr, precOr, false, false}, true
		case "xor":
			return binOp{ast.OpXor, precXor, false, false}, true
		case "and":
			return binOp{ast.OpConditionalAnd, precAnd, false, false}, true
		case "hastype":
			return binOp{ast.OpHasType, precClassify, false, true}, true
		case "istype":
			return binOp{ast.OpIsType, precClassify, false, true}, true
		case "as":
			return binOp{ast.OpAs, precClassify, false, true}, true
		case "meta":
			return binOp{ast.OpMeta, precClassify, false, true}, true
		}
	}
	return binOp{}, false
}

// parseBinary parses a binary expression at or above the given precedence.
func (p *Parser) parseBinary(minPrec int) ast.Node {
	start := p.peek().Span.Offset
	left := p.parseUnary()
	for {
		bop, ok := p.binaryOpFor()
		if !ok || bop.prec < minPrec {
			break
		}
		p.advance() // operator
		e := &ast.OperatorExpr{Operator: bop.op}
		if bop.classify {
			e.Operands = []ast.Node{left}
			e.TypeRef = p.parseQualifiedName()
		} else {
			nextMin := bop.prec + 1
			if bop.rightAssoc {
				nextMin = bop.prec
			}
			right := p.parseBinary(nextMin)
			e.Operands = []ast.Node{left, right}
		}
		e.NodeSpan = p.spanFrom(start)
		left = e
	}
	return left
}

// parseUnary parses prefix operators and the `all` extent, then a primary.
func (p *Parser) parseUnary() ast.Node {
	start := p.peek().Span.Offset
	var op ast.OperatorKind
	switch {
	case p.at(lexer.Plus):
		op = ast.OpPos
	case p.at(lexer.Minus):
		op = ast.OpNeg
	case p.at(lexer.Tilde):
		op = ast.OpBitNot
	case p.atKeyword("not"):
		op = ast.OpNot
	case p.atKeyword("all"):
		p.advance()
		operand := p.parseUnary()
		e := &ast.OperatorExpr{Operator: ast.OpAll, Operands: []ast.Node{operand}}
		e.NodeSpan = p.spanFrom(start)
		return e
	default:
		return p.parsePrimary()
	}
	p.advance() // prefix operator
	operand := p.parseUnary()
	e := &ast.OperatorExpr{Operator: op, Operands: []ast.Node{operand}}
	e.NodeSpan = p.spanFrom(start)
	return e
}

// expect2Keyword records a diagnostic if the given keyword is not present,
// consuming it when it is.
func (p *Parser) expect2Keyword(kw string) bool {
	if p.acceptKeyword(kw) {
		return true
	}
	p.error(p.peek().Span, "expected '"+kw+"'")
	return false
}

// parsePrimary parses a base expression and then any chain of postfix
// operators (feature chain, index, invocation, collect, select).
func (p *Parser) parsePrimary() ast.Node {
	start := p.peek().Span.Offset
	expr := p.parseBase()
	return p.parsePostfixes(start, expr)
}

// metadataAccessRef is the element reference `ref.metadata` reads the metadata
// of; nothing but a name is one (KerMLExpressions MetadataAccessExpression).
func metadataAccessRef(expr ast.Node) *ast.QualifiedName {
	switch e := expr.(type) {
	case *ast.FeatureReference:
		return e.Name
	case *ast.QualifiedName:
		return e
	default:
		return nil
	}
}

// atExprStart reports whether the current token can start an expression.
func (p *Parser) atExprStart() bool {
	t := p.peek()
	return p.atName() ||
		t.Kind == lexer.Decimal ||
		t.Kind == lexer.Real ||
		t.Kind == lexer.String ||
		t.Kind == lexer.Star || // infinity
		t.Kind == lexer.LParen ||
		t.Kind == lexer.LBrace ||
		(t.Kind == lexer.Keyword && exprStartKeywords[t.KeywordID])
}

// atPrefixOperator reports whether the current token is a prefix operator that
// parseUnary reads (`-x`, `not x`).
func (p *Parser) atPrefixOperator() bool {
	t := p.peek()
	return t.Kind == lexer.Plus || t.Kind == lexer.Minus || t.Kind == lexer.Tilde ||
		p.atKeyword("not")
}

// atUnaryExprStart reports whether the current token can start an expression,
// a prefix operator included. The bare `atExprStart` is the narrower set an
// argument written without parentheses admits.
func (p *Parser) atUnaryExprStart() bool {
	return p.atExprStart() || p.atPrefixOperator()
}

// parsePostfixes applies zero or more postfix operators to expr.
func (p *Parser) parsePostfixes(start int, expr ast.Node) ast.Node {
	for {
		switch {
		case p.at(lexer.Dot):
			// `.member` (chain) or `.{ body }` (collect).
			p.advance() // .
			if p.at(lexer.LBrace) {
				body := p.parseBodyExpr(p.peek().Span.Offset)
				c := &ast.CollectExpr{Operand: expr, Body: body}
				c.NodeSpan = p.spanFrom(start)
				expr = c
				continue
			}
			// `ref.metadata` is a MetadataAccessExpression over an element
			// reference, not a chain through a feature named `metadata`.
			if p.atKeyword("metadata") {
				if ref := metadataAccessRef(expr); ref != nil {
					p.advance() // metadata
					m := &ast.MetadataAccessExpr{Ref: ref}
					m.NodeSpan = p.spanFrom(start)
					expr = m
					continue
				}
			}
			// Use relaxed parsing to allow keywords as feature names (e.g., oSP.exit, state.entry)
			member := p.parseQualifiedNameRelaxed()
			fc := &ast.FeatureChainExpr{Operand: expr, Member: member}
			fc.NodeSpan = p.spanFrom(start)
			expr = fc
			// `f.s(1)`: the instantiated type of an invocation may be a feature
			// chain (KerMLExpressions InstantiatedTypeMember → OwnedFeatureChain).
			if p.at(lexer.LParen) {
				expr = p.parseInvocationTail(start, fc, nil)
			}

		case p.at(lexer.DotQuestion):
			// `.?{ body }` (select). The body is a body expression whatever it
			// declares, so `.?{in x; x > 0}` and a bare `.?{...}` parse alike and
			// the parameters of either are the parameters of one node kind.
			p.advance() // .?
			if !p.at(lexer.LBrace) {
				p.error(p.peek().Span, "expected '{' after '.?'")
				return expr
			}
			body := p.parseBodyExpr(p.peek().Span.Offset)
			s := &ast.SelectExpr{Operand: expr, Body: body}
			s.NodeSpan = p.spanFrom(start)
			expr = s

		case p.at(lexer.Hash):
			// `#( index )` sequence index. The index is a SequenceExpression,
			// so a multi-dimensional index `arr#(1,3)` is one operand.
			p.advance() // #
			p.expect(lexer.LParen, "expected '(' after '#'")
			idx := p.parseSequenceExpr()
			p.expect(lexer.RParen, msgExpectedCloseParen)
			ix := &ast.IndexExpr{Operand: expr, Index: idx}
			ix.NodeSpan = p.spanFrom(start)
			expr = ix

		case p.at(lexer.LBracket):
			// `[ index ]` operator index.
			p.advance() // [
			idx := p.parseSequenceExpr()
			p.expect(lexer.RBracket, "expected ']'")
			ix := &ast.IndexExpr{Operand: expr, Index: idx, Bracket: true}
			ix.NodeSpan = p.spanFrom(start)
			expr = ix

		case p.at(lexer.Arrow):
			// `-> Type ( args )` invocation with receiver.
			p.advance() // ->
			typ := p.parseQualifiedName()
			inv := &ast.InvocationExpr{Operand: expr, Type: typ}
			if p.at(lexer.LParen) {
				inv.Args, inv.NamedArgs = p.parseArgList()
			} else if p.at(lexer.LBrace) {
				// Function reference given as a body: store as a single arg.
				inv.Args = []ast.Node{p.parseBodyExpr(p.peek().Span.Offset)}
			} else if p.atExprStart() {
				// Single arg without parens (e.g., `reduce '*'`)
				// Parse only base expression, not full precedence expression
				inv.Args = []ast.Node{p.parseBase()}
			}
			inv.NodeSpan = p.spanFrom(start)
			expr = inv

		default:
			return expr
		}
	}
}

// parseBase parses a leaf/base expression.
func (p *Parser) parseBase() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()

	setBase := func(n ast.Node) ast.Node {
		if nb, ok := n.(interface{ SetLeadingTrivia([]ast.Trivia) }); ok {
			nb.SetLeadingTrivia(trivia)
		}
		return n
	}

	switch {
	case p.atKeyword("null"):
		p.advance()
		n := &ast.NullExpr{}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.atKeyword("true"), p.atKeyword("false"):
		tok := p.advance()
		n := &ast.LiteralBool{Value: tok.KeywordID == "true"}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.atKeyword("new"):
		return setBase(p.parseConstructor(start))

	case p.at(lexer.Decimal):
		tok := p.advance()
		n := &ast.LiteralInteger{Value: p.src.Text(tok.Span)}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.Real):
		tok := p.advance()
		n := &ast.LiteralReal{Value: p.src.Text(tok.Span)}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.String):
		tok := p.advance()
		n := &ast.LiteralString{Value: p.src.Text(tok.Span)}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.Star):
		// Infinity literal in expression position.
		p.advance()
		n := &ast.LiteralInfinity{}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.LParen):
		return setBase(p.parseParenOrSequence(start))

	case p.at(lexer.LBrace):
		return setBase(p.parseBodyExpr(start))

	case p.at(lexer.At), p.at(lexer.AtAt):
		// A classification expression whose operand is left implicit:
		// `@MetadataType` and `@@Metaclass` (KerML.xtext ClassificationExpression
		// and MetaclassificationExpression both make the tested operand
		// optional). The omitted operand is the element the expression is
		// evaluated for — `self` — which an element filter supplies as the
		// candidate element, so Operands stays empty rather than holding a
		// synthesized reference to a name that is not in scope.
		op := ast.OpAt
		if p.at(lexer.AtAt) {
			op = ast.OpMetaAt
		}
		p.advance() // consume '@' or '@@'
		e := &ast.OperatorExpr{Operator: op, TypeRef: p.parseQualifiedName()}
		e.NodeSpan = p.spanFrom(start)
		return setBase(e)

	case p.atName(), p.at(lexer.Keyword):
		// Parse qualified name or keyword-as-name
		var qn *ast.QualifiedName
		if p.at(lexer.Keyword) {
			// Keywords can be used as feature references (e.g., `excluding(do)`)
			tok := p.advance()
			seg := ast.NameSegment{Text: tok.KeywordID, Span: tok.Span}
			qn = &ast.QualifiedName{Parts: []ast.NameSegment{seg}}
			qn.NodeSpan = tok.Span
		} else {
			qn = p.parseQualifiedName()
		}
		// A bare `Type(args)` invocation with no receiver is recognized here.
		if p.at(lexer.LParen) {
			return setBase(p.parseInvocationTail(start, nil, qn))
		}
		// Handle body expression invocations: `forAll { in i; expr }`
		// Only if LBrace followed by 'in' keyword (body expression parameter)
		if p.at(lexer.LBrace) && p.peekN(1).Kind == lexer.Keyword && p.peekN(1).KeywordID == "in" {
			bodyStart := p.peek().Span.Offset
			bodyExpr := p.parseBodyExpr(bodyStart)
			inv := &ast.InvocationExpr{Type: qn, Args: []ast.Node{bodyExpr}}
			inv.NodeSpan = p.spanFrom(start)
			return setBase(inv)
		}
		fr := &ast.FeatureReference{Name: qn}
		fr.NodeSpan = p.spanFrom(start)
		return setBase(fr)

	default:
		p.error(p.peek().Span, "expected an expression")
		en := &ast.ErrorNode{Message: "expected an expression"}
		if !p.atEOF() && !p.at(lexer.RParen) && !p.at(lexer.RBrace) && !p.at(lexer.Semicolon) {
			p.advance() // ensure progress
		}
		en.NodeSpan = p.spanFrom(start)
		return setBase(en)
	}
}

// parseParenOrSequence parses `( )` (the null expression), `( expr )`, or
// `( expr, expr, ... )`.
func (p *Parser) parseParenOrSequence(start int) ast.Node {
	p.advance() // (

	// Check for cast expression: (as Type) or (as Type[mult])
	if p.atKeyword("as") {
		p.advance() // consume 'as'
		targetType := p.parseQualifiedName()
		var mult *ast.Multiplicity
		if p.at(lexer.LBracket) {
			mult = p.parseMultiplicity()
		}
		p.expect(lexer.RParen, "expected ')' after cast type")
		cast := &ast.CastExpr{
			TargetType:   targetType,
			Multiplicity: mult,
		}
		cast.NodeSpan = p.spanFrom(start)
		return cast
	}

	// Regular parenthesized expression or sequence
	var elems []ast.Node
	if !p.at(lexer.RParen) {
		elems = append(elems, p.ParseExpression())
		for p.at(lexer.Comma) {
			p.advance() // ,
			elems = append(elems, p.ParseExpression())
		}
	}
	p.expect(lexer.RParen, msgExpectedCloseParen)
	switch len(elems) {
	case 0:
		null := &ast.NullExpr{}
		null.NodeSpan = p.spanFrom(start)
		return null
	case 1:
		return elems[0]
	}
	seq := &ast.SequenceExpr{Elements: elems}
	seq.NodeSpan = p.spanFrom(start)
	return seq
}

// parseSequenceExpr parses a KerMLExpressions SequenceExpression: one
// expression, or a comma-separated sequence of them (`1, 3`), as one node.
func (p *Parser) parseSequenceExpr() ast.Node {
	start := p.peek().Span.Offset
	elems := []ast.Node{p.ParseExpression()}
	for p.at(lexer.Comma) {
		p.advance() // ,
		elems = append(elems, p.ParseExpression())
	}
	if len(elems) == 1 {
		return elems[0]
	}
	seq := &ast.SequenceExpr{Elements: elems}
	seq.NodeSpan = p.spanFrom(start)
	return seq
}

// parseConstructor parses `new QualifiedName ( args )`.
func (p *Parser) parseConstructor(start int) ast.Node {
	p.advance() // new
	qn := p.parseQualifiedName()
	c := &ast.ConstructorExpr{Type: qn}
	if p.at(lexer.LParen) {
		c.Args, c.NamedArgs = p.parseArgList()
	}
	c.NodeSpan = p.spanFrom(start)
	return c
}

// parseArgList parses `( )`, positional `( a, b )`, or named `( n=a, m=b )`.
// Returns positional args and named args (one slice empty).
func (p *Parser) parseArgList() ([]ast.Node, []ast.NamedArg) {
	p.expect(lexer.LParen, "expected '('")
	var pos []ast.Node
	var named []ast.NamedArg
	if p.at(lexer.RParen) {
		p.advance()
		return pos, named
	}
	// Named if the first token is a name immediately followed by '='.
	if p.namedArgAhead() {
		for {
			name := p.parseQualifiedName()
			p.expect(lexer.Eq, "expected '=' in named argument")
			val := p.ParseExpression()
			named = append(named, ast.NamedArg{Name: name, Value: val})
			if !p.at(lexer.Comma) {
				break
			}
			p.advance()
		}
	} else {
		for {
			pos = append(pos, p.ParseExpression())
			if !p.at(lexer.Comma) {
				break
			}
			p.advance()
		}
	}
	p.expect(lexer.RParen, msgExpectedCloseParen)
	return pos, named
}

// namedArgAhead reports whether the arg list is `name = ...` (named form).
func (p *Parser) namedArgAhead() bool {
	if !p.atName() {
		return false
	}
	// Skip a qualified name, then check for '='.
	i := 1
	for p.peekN(i).Kind == lexer.ColonColon {
		i++
		if k := p.peekN(i).Kind; k != lexer.Identifier && k != lexer.UnrestrictedName {
			return false
		}
		i++
	}
	return p.peekN(i).Kind == lexer.Eq
}

// parseInvocationTail parses `( args )` after a receiver/type has been read.
func (p *Parser) parseInvocationTail(start int, recv ast.Node, typ *ast.QualifiedName) ast.Node {
	args, named := p.parseArgList()
	inv := &ast.InvocationExpr{Operand: recv, Type: typ, Args: args, NamedArgs: named}
	inv.NodeSpan = p.spanFrom(start)
	return inv
}

// atBodyExprMember reports whether a declaration follows in a body expression,
// rather than its result expression: a visibility or a kind keyword introduces
// one (`private attribute lbcf = …;`), neither of which starts an expression.
func (p *Parser) atBodyExprMember() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	switch p.peek().KeywordID {
	case "public", "private", "protected":
		return true
	}
	if !p.isKindKeyword(p.peek()) {
		return false
	}
	// A kind keyword is the kind only when the name of a declaration follows;
	// otherwise the result expression reads it as a name (`objective(a)`).
	switch p.peekN(1).Kind {
	case lexer.Identifier, lexer.UnrestrictedName, lexer.Lt:
		return true
	}
	return false
}

// parseBodyExpr parses `{ [doc] (in param ;)* (member)* resultExpr }`.
func (p *Parser) parseBodyExpr(start int) ast.Node {
	p.advance() // {
	defer p.pushBodyContext(bodyOther)()
	b := &ast.BodyExpr{}

	// A body may open with documentation, a member of the body like its features.
	if p.atKeyword("doc") {
		b.Members = append(b.Members, p.parseDocumentation(p.peek().Span.Offset))
	}

	// Check for shorthand param syntax: {name : Type; expr} without "in" keyword
	// Common in collection operators like ->exists{p : Point; condition}
	hasShorthandParam := false
	if p.atName() && p.peekN(1).Kind == lexer.Colon {
		hasShorthandParam = true
	}

	if hasShorthandParam {
		// Parse single param without "in" keyword
		var paramType *ast.QualifiedName
		var paramMult *ast.Multiplicity

		if seg, ok := p.parseNameSegment(); ok {
			if p.at(lexer.Colon) {
				p.advance() // :
				paramType = p.parseQualifiedName()
				// Parse optional multiplicity after type
				if p.at(lexer.LBracket) {
					paramMult = p.parseMultiplicity()
				}
			}
			b.Params = append(b.Params, ast.BodyParam{
				Name:         seg.Text,
				Type:         paramType,
				Multiplicity: paramMult,
				Span:         seg.Span,
			})
		}
		p.expectSemicolon("body parameter")
	}

	for p.atKeyword("in") || p.atBodyExprMember() {
		// A body expression is a calculation body, so it may declare features of
		// its own between its parameters and its result.
		if !p.atKeyword("in") {
			before := p.peek().Span.Offset
			b.Members = append(b.Members, p.parseBodyMember())
			// Force progress: a member that consumed nothing would spin the loop.
			if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
				p.advance()
			}
			continue
		}
		p.advance() // in
		var paramType *ast.QualifiedName
		var paramMult *ast.Multiplicity
		var paramValue ast.Node
		var isRef bool

		// Check for 'ref' modifier after 'in'
		if p.atKeyword("ref") {
			p.advance()
			isRef = true
		}

		seg, ok := p.parseNameSegment()
		if !ok {
			// `in` declares a parameter, and a parameter is named: a body whose
			// parameter has no name declares nothing its result could read, so
			// the notation is reported rather than parsed as a body of none.
			p.error(p.peek().Span, "expected a name after 'in' in a body parameter")
		}
		if ok {
			var paramMembers []ast.Node
			if p.at(lexer.Colon) {
				p.advance() // :
				paramType = p.parseQualifiedName()
				// Parse optional multiplicity after type
				if p.at(lexer.LBracket) {
					paramMult = p.parseMultiplicity()
				}
			}
			// A parameter may specialize a feature instead of naming a type
			// (`in p :> ISQ::mass`), which is how a filter names the feature its
			// elements redefine.
			paramRels := p.parseRelationships(true)
			if _, ok := p.accept(lexer.Eq); ok {
				paramValue = p.ParseExpression()
			}
			// Parse optional body members: in ref a { doc ... }
			if p.at(lexer.LBrace) {
				p.advance() // {
				leave := p.pushBodyContext(bodyOther)
				for !p.at(lexer.RBrace) && !p.atEOF() {
					paramMembers = append(paramMembers, p.parseBodyMember())
				}
				leave()
				p.expect(lexer.RBrace, "expected '}'")
			}
			b.Params = append(b.Params, ast.BodyParam{
				Name:          seg.Text,
				Type:          paramType,
				Multiplicity:  paramMult,
				Value:         paramValue,
				IsReference:   isRef,
				Members:       paramMembers,
				Relationships: paramRels,
				Span:          seg.Span,
			})
		}
		// No semicolon expected if param has body
		if len(b.Params) == 0 || len(b.Params[len(b.Params)-1].Members) == 0 {
			p.expectSemicolon("body parameter")
		}
	}
	if !p.at(lexer.RBrace) {
		b.Result = p.ParseExpression()
	}
	p.expect(lexer.RBrace, "expected '}'")
	b.NodeSpan = p.spanFrom(start)
	return b
}
