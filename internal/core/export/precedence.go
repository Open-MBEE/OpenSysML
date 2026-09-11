package export

// The structural expression writer places parentheses by how tightly each form
// binds — the parser's precedence table — so the notation reads back as the tree.

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// The forms of an expression, loosest binding first.
const (
	bindConditional    = iota // if ? else
	bindNullCoalesce          // ??
	bindImplies               // implies
	bindOr                    // | or
	bindXor                   // xor
	bindAnd                   // & and
	bindEquality              // == != === !==
	bindClassify              // hastype istype @ @@ as meta
	bindRelational            // < > <= >=
	bindRange                 // ..
	bindAdditive              // + -
	bindMultiplicative        // * / %
	bindExponent              // ** ^, grouping to the right
	bindUnary                 // prefix + - ~ not, and all
	bindPrimary               // literals, names, postfix and bracketed forms
)

// infixBinding is how tightly each infix operator spelling binds.
var infixBinding = map[string]int{
	"??": bindNullCoalesce, "implies": bindImplies, "|": bindOr, "or": bindOr, "xor": bindXor,
	"&": bindAnd, "and": bindAnd, "==": bindEquality, "!=": bindEquality, "===": bindEquality, "!==": bindEquality,
	"hastype": bindClassify, "istype": bindClassify, "@": bindClassify, "@@": bindClassify, "as": bindClassify, "meta": bindClassify,
	"<": bindRelational, ">": bindRelational, "<=": bindRelational, ">=": bindRelational,
	"..": bindRange, "+": bindAdditive, "-": bindAdditive, "*": bindMultiplicative, "/": bindMultiplicative, "%": bindMultiplicative,
	"**": bindExponent, "^": bindExponent,
}

// prefixOperators are the operator spellings written ahead of one operand.
var prefixOperators = map[string]bool{"+": true, "-": true, "~": true, "not": true, "all": true}

// A multiplicity bound is read above range precedence, so `..` stays the
// separator of the bounds; every other position reads a whole expression.
func positionBinding(property string) int {
	switch property {
	case pLowerBound, pUpperBound:
		return bindAdditive
	}
	return bindConditional
}

// operand is an expression as notation, with how tightly that notation binds.
type operand struct {
	text    string
	binding int
	// elements is the comma-separated list a sequence operand encloses.
	elements string
}

// at writes the operand where it must bind at least as tightly as floor.
func (o operand) at(floor int) string {
	if o.binding < floor {
		return "(" + o.text + ")"
	}
	return o.text
}

// notationBinding is how tightly a kept notation binds, read by parsing it. Text
// that does not read as one expression binds loosest, so it is always enclosed.
func notationBinding(text string) int {
	text = strings.TrimSpace(text)
	p := parser.New(source.New("<expression>", []byte(text)))
	node := p.ParseExpression()
	if len(p.Diagnostics) > 0 || node == nil || node.Span().End() != len(text) {
		return bindConditional
	}
	return nodeBinding(node)
}

// nodeBinding is how tightly a parsed expression binds, judged by its outermost
// form; a form the parser only reads between brackets is a primary.
func nodeBinding(node ast.Node) int {
	switch n := node.(type) {
	case *ast.OperatorExpr:
		switch {
		case n.Operator == ast.OpConditional:
			return bindConditional
		case len(n.Operands) == 0:
			return bindPrimary
		case len(n.Operands) == 1 && n.TypeRef == nil:
			return bindUnary
		}
		if binding, ok := infixBinding[n.Operator.String()]; ok {
			return binding
		}
		return bindConditional
	}
	return bindPrimary
}
