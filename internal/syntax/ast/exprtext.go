package ast

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ExprPrinter spells an expression as the notation writes it, an operand parenthesised only
// where its operator binds looser than the enclosing one, so the text reads back as the tree.
type ExprPrinter struct {
	// Unit spells the measurement reference of a quantity `1.0 [m]`; nil or "" spells it as an expression.
	Unit func(Node) string
	// Other spells a node of a form the printer has no notation for; nil spells it "".
	Other func(Node) string
}

// Text spells node; "" when node, or an operand of it, has no spelling.
func (p ExprPrinter) Text(node Node) string {
	switch expr := node.(type) {
	case *LiteralInteger:
		return expr.Value
	case *LiteralReal:
		return expr.Value
	case *LiteralString:
		return expr.Value
	case *LiteralBool:
		if expr.Value {
			return "true"
		}
		return "false"
	case *LiteralInfinity:
		return "*"
	case *NullExpr:
		return "null"
	case *QualifiedName:
		return QualifiedNameText(expr)
	case *FeatureReference:
		return QualifiedNameText(expr.Name)
	case *FeatureChainExpr:
		return p.join(p.primary(expr.Operand), ".", QualifiedNameText(expr.Member))
	case *OperatorExpr:
		return p.operator(expr)
	case *InvocationExpr:
		return p.invocation(expr)
	case *IndexExpr:
		if expr.Bracket {
			return p.join(p.primary(expr.Operand), " [", p.unit(expr.Index), "]")
		}
		return p.join(p.primary(expr.Operand), "#(", p.Text(expr.Index), ")")
	}
	if p.Other == nil {
		return ""
	}
	return p.Other(node)
}

// QualifiedNameText writes a qualified name as the notation does: `$::` on a global one,
// each segment quoted when it needs quoting, a chained segment after `.`.
func QualifiedNameText(qn *QualifiedName) string {
	if qn == nil {
		return ""
	}
	var sb strings.Builder
	if qn.Global {
		sb.WriteString("$::")
	}
	for i, part := range qn.Parts {
		switch {
		case i == 0:
		case part.Chained:
			sb.WriteString(".")
		default:
			sb.WriteString("::")
		}
		sb.WriteString(source.NameText(part.Text))
	}
	return sb.String()
}

// join concatenates parts, spelling "" when any part is "".
func (p ExprPrinter) join(parts ...string) string {
	for _, part := range parts {
		if part == "" {
			return ""
		}
	}
	return strings.Join(parts, "")
}

func (p ExprPrinter) unit(index Node) string {
	if p.Unit != nil {
		if text := p.Unit(index); text != "" {
			return text
		}
	}
	return p.Text(index)
}

// primary spells the operand of a postfix (a chain, an index, a receiver), grouped when it
// is an operator application, which binds looser than any postfix.
func (p ExprPrinter) primary(operand Node) string {
	text := p.Text(operand)
	if _, ok := operand.(*OperatorExpr); ok && text != "" {
		return "(" + text + ")"
	}
	return text
}

// operator spells a unary operator before its operand, a binary one between its two, a
// conditional as `if c ? a else b`, a classification or cast with its type.
func (p ExprPrinter) operator(expr *OperatorExpr) string {
	operands := make([]string, len(expr.Operands))
	for i, operand := range expr.Operands {
		operands[i] = p.operand(expr, i, operand)
		if operands[i] == "" {
			return ""
		}
	}
	op := expr.Operator.String()
	switch {
	case expr.Operator == OpConditional && len(operands) == 3:
		return "if " + operands[0] + " ? " + operands[1] + " else " + operands[2]
	case expr.TypeRef != nil && len(operands) == 1:
		return p.join(operands[0], " ", op, " ", QualifiedNameText(expr.TypeRef))
	case len(operands) == 1 && (expr.Operator == OpNot || expr.Operator == OpAll):
		return op + " " + operands[0]
	case len(operands) == 1:
		return op + operands[0]
	case len(operands) == 2:
		return operands[0] + " " + op + " " + operands[1]
	}
	return ""
}

// operand spells the i-th operand of expr, parenthesised when it is an
// operator application the parser would otherwise attach differently.
func (p ExprPrinter) operand(expr *OperatorExpr, i int, operand Node) string {
	text := p.Text(operand)
	inner, ok := operand.(*OperatorExpr)
	if !ok || text == "" {
		return text
	}
	if grouped(expr, i, inner) {
		return "(" + text + ")"
	}
	return text
}

// grouped reports whether inner, the i-th operand of expr, needs parentheses: one binding
// looser than expr, or as tight on the side expr does not associate to (`**` to the right).
func grouped(expr *OperatorExpr, i int, inner *OperatorExpr) bool {
	if expr.Operator == OpConditional {
		return i == 0 && inner.Operator == OpConditional
	}
	outer, in := operatorPrecedence(expr), operatorPrecedence(inner)
	if len(expr.Operands) == 1 && expr.TypeRef == nil {
		return in < outer
	}
	return in < outer || in == outer && (i == 1) != (expr.Operator == OpPow)
}

// operatorPrecedence ranks an application as the KerML expression grammar binds its
// operator, loosest first; a unary application binds tightest.
func operatorPrecedence(expr *OperatorExpr) int {
	switch expr.Operator {
	case OpConditional:
		return 0
	case OpNullCoalesce:
		return 1
	case OpImplies:
		return 2
	case OpOr, OpConditionalOr:
		return 3
	case OpXor:
		return 4
	case OpAnd, OpConditionalAnd:
		return 5
	case OpEq, OpNeq, OpEqEqEq, OpNeqEqEq:
		return 6
	case OpHasType, OpIsType, OpAt, OpMetaAt, OpAs, OpMeta:
		return 7
	case OpLt, OpGt, OpLe, OpGe:
		return 8
	case OpRange:
		return 9
	case OpAdd, OpSub:
		return 10
	case OpMul, OpDiv, OpMod:
		return 11
	case OpPow:
		return 12
	}
	return 13
}

// invocation spells `Type(args)`, `operand->Type(args)` or the bare-argument `operand->Type arg`,
// a named argument as `name = value`.
func (p ExprPrinter) invocation(expr *InvocationExpr) string {
	args := make([]string, 0, len(expr.Args)+len(expr.NamedArgs))
	for _, arg := range expr.Args {
		args = append(args, p.Text(arg))
	}
	for _, arg := range expr.NamedArgs {
		args = append(args, p.join(QualifiedNameText(arg.Name), " = ", p.Text(arg.Value)))
	}
	for _, arg := range args {
		if arg == "" {
			return ""
		}
	}
	callee := QualifiedNameText(expr.Type)
	if expr.Type == nil {
		callee = p.primary(expr.Operand)
	} else if expr.Operand != nil {
		callee = p.join(p.primary(expr.Operand), "->", callee)
	}
	if expr.BareArgument && len(args) == 1 {
		return p.join(callee, " ", args[0])
	}
	return p.join(callee, "(") + strings.Join(args, ", ") + ")"
}
