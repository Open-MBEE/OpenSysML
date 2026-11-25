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
	case *CollectExpr:
		return p.join(p.primary(expr.Operand), ".", p.Text(expr.Body))
	case *SelectExpr:
		return p.join(p.primary(expr.Operand), ".?", p.Text(expr.Body))
	case *ConstructorExpr:
		return p.join("new ", QualifiedNameText(expr.Type), p.arguments(expr.Args, expr.NamedArgs))
	case *BodyExpr:
		if text := p.body(expr); text != "" {
			return text
		}
	case *SequenceExpr:
		return p.join("(", p.list(expr.Elements), ")")
	case *MetadataAccessExpr:
		return p.join(QualifiedNameText(expr.Ref), ".metadata")
	case *CastExpr:
		if expr.Multiplicity == nil {
			return p.join("(as ", QualifiedNameText(expr.TargetType), ")")
		}
		return p.join("(as ", QualifiedNameText(expr.TargetType), p.multiplicity(expr.Multiplicity), ")")
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
// conditional as `if c ? a else b`, a classification or cast with its type (`@T` over self).
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
	case expr.TypeRef != nil && len(operands) == 0:
		return p.join(op, QualifiedNameText(expr.TypeRef))
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
	if len(inner.Operands) == 0 {
		return false
	}
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
	callee := QualifiedNameText(expr.Type)
	if expr.Type == nil {
		callee = p.primary(expr.Operand)
	} else if expr.Operand != nil {
		callee = p.join(p.primary(expr.Operand), "->", callee)
	}
	if expr.BareArgument && len(expr.Args) == 1 && len(expr.NamedArgs) == 0 {
		return p.join(callee, " ", p.Text(expr.Args[0]))
	}
	return p.join(callee, p.arguments(expr.Args, expr.NamedArgs))
}

// arguments spells a parenthesised argument list, a named argument as `name = value`.
func (p ExprPrinter) arguments(positional []Node, named []NamedArg) string {
	args := make([]string, 0, len(positional)+len(named))
	for _, arg := range positional {
		args = append(args, p.Text(arg))
	}
	for _, arg := range named {
		args = append(args, p.join(QualifiedNameText(arg.Name), " = ", p.Text(arg.Value)))
	}
	for _, arg := range args {
		if arg == "" {
			return ""
		}
	}
	return "(" + strings.Join(args, ", ") + ")"
}

// list spells nodes separated by commas; "" when there is none or one has no spelling.
func (p ExprPrinter) list(nodes []Node) string {
	texts := make([]string, len(nodes))
	for i, node := range nodes {
		if texts[i] = p.Text(node); texts[i] == "" {
			return ""
		}
	}
	return strings.Join(texts, ", ")
}

// body spells `{in x : T; result}`; "" for a body declaring members, or a parameter
// with more than a type and a default, which only the declaration printer spells.
func (p ExprPrinter) body(expr *BodyExpr) string {
	if len(expr.Members) > 0 || expr.Result == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("{")
	for _, param := range expr.Params {
		if param.Multiplicity != nil || len(param.Members) > 0 || len(param.Relationships) > 0 {
			return ""
		}
		sb.WriteString("in ")
		if param.IsReference {
			sb.WriteString("ref ")
		}
		sb.WriteString(source.NameText(param.Name))
		if param.Type != nil {
			sb.WriteString(" : " + QualifiedNameText(param.Type))
		}
		if param.Value != nil {
			value := p.Text(param.Value)
			if value == "" {
				return ""
			}
			sb.WriteString(" = " + value)
		}
		sb.WriteString("; ")
	}
	result := p.Text(expr.Result)
	if result == "" {
		return ""
	}
	return sb.String() + result + "}"
}

// multiplicity spells `[n]` or `[lower..upper]`.
func (p ExprPrinter) multiplicity(m *Multiplicity) string {
	if m.IsRange {
		return p.join("[", p.Text(m.Lower), "..", p.Text(m.Upper), "]")
	}
	return p.join("[", p.Text(m.Lower), "]")
}
