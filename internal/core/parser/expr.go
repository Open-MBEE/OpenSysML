package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
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
		thn := p.parseBinary(precNullCoalesce)
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
	t := p.peek()
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
		p.atKeyword("null") || 
		p.atKeyword("true") || 
		p.atKeyword("false") || 
		p.atKeyword("new") ||
		p.atKeyword("if")
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
			// Use relaxed parsing to allow keywords as feature names (e.g., oSP.exit, state.entry)
			member := p.parseQualifiedNameRelaxed()
			fc := &ast.FeatureChainExpr{Operand: expr, Member: member}
			fc.NodeSpan = p.spanFrom(start)
			expr = fc

		case p.at(lexer.DotQuestion):
			// `.?{ body }` (select).
			p.advance() // .?
			body := p.parseBodyExpr(p.peek().Span.Offset)
			s := &ast.SelectExpr{Operand: expr, Body: body}
			s.NodeSpan = p.spanFrom(start)
			expr = s

		case p.at(lexer.Hash):
			// `#( index )` sequence index.
			p.advance() // #
			p.expect(lexer.LParen, "expected '(' after '#'")
			idx := p.ParseExpression()
			p.expect(lexer.RParen, "expected ')'")
			ix := &ast.IndexExpr{Operand: expr, Index: idx}
			ix.NodeSpan = p.spanFrom(start)
			expr = ix

		case p.at(lexer.LBracket):
			// `[ index ]` operator index.
			p.advance() // [
			idx := p.ParseExpression()
			p.expect(lexer.RBracket, "expected ']'")
			ix := &ast.IndexExpr{Operand: expr, Index: idx}
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

// parseParenOrSequence parses `( )`, `( expr )`, or `( expr, expr, ... )`.
func (p *Parser) parseParenOrSequence(start int) ast.Node {
	p.advance() // (
	var elems []ast.Node
	if !p.at(lexer.RParen) {
		elems = append(elems, p.ParseExpression())
		for p.at(lexer.Comma) {
			p.advance() // ,
			elems = append(elems, p.ParseExpression())
		}
	}
	p.expect(lexer.RParen, "expected ')'")
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
		c.Args, _ = p.parseArgList()
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
	p.expect(lexer.RParen, "expected ')'")
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

// parseBodyExpr parses `{ [doc] (in param ;)* resultExpr }`.
func (p *Parser) parseBodyExpr(start int) ast.Node {
	p.advance() // {
	b := &ast.BodyExpr{}
	
	// Parse optional doc comment at start of body expression
	if p.atKeyword("doc") {
		// Skip doc comment - not stored in BodyExpr AST node
		// Doc is part of the body expression context but not the expression itself
		p.parseDocumentation(p.peek().Span.Offset)
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
		p.expect(lexer.Semicolon, "expected ';' after body parameter")
	}
	
	for p.atKeyword("in") {
		p.advance() // in
		var paramType *ast.QualifiedName
		var paramMult *ast.Multiplicity
		var paramValue ast.Node
		var isRef bool
		var redefinesTarget *ast.QualifiedName
		
		// Check for 'ref' modifier after 'in'
		if p.atKeyword("ref") {
			p.advance()
			isRef = true
		}
		
		if seg, ok := p.parseNameSegment(); ok {
			var paramMembers []ast.Node
			if p.at(lexer.Colon) {
				p.advance() // :
				paramType = p.parseQualifiedName()
				// Parse optional multiplicity after type
				if p.at(lexer.LBracket) {
					paramMult = p.parseMultiplicity()
				}
			} else if p.at(lexer.ColonGt) || p.at(lexer.ColonGtGt) {
				// Redefines relationship: in p :> Parent or in p :>> Parent
				p.advance() // :> or :>>
				redefinesTarget = p.parseQualifiedName()
			}
			if p.at(lexer.Eq) {
				p.advance() // =
				paramValue = p.ParseExpression()
			}
			// Parse optional body members: in ref a { doc ... }
			if p.at(lexer.LBrace) {
				p.advance() // {
				for !p.at(lexer.RBrace) && !p.atEOF() {
					paramMembers = append(paramMembers, p.parseBodyMember())
				}
				p.expect(lexer.RBrace, "expected '}'")
			}
			b.Params = append(b.Params, ast.BodyParam{
				Name:            seg.Text,
				Type:            paramType,
				RedefinesTarget: redefinesTarget,
				Multiplicity:    paramMult,
				Value:           paramValue,
				IsReference:     isRef,
				Members:         paramMembers,
				Span:            seg.Span,
			})
		}
		// No semicolon expected if param has body
		if len(b.Params) == 0 || len(b.Params[len(b.Params)-1].Members) == 0 {
			p.expect(lexer.Semicolon, "expected ';' after body parameter")
		}
	}
	if !p.at(lexer.RBrace) {
		b.Result = p.ParseExpression()
	}
	p.expect(lexer.RBrace, "expected '}'")
	b.NodeSpan = p.spanFrom(start)
	return b
}
