package ast

import (
	"fmt"
	"sort"
)

// ValuePartOf is the value part a declaration writes (KerML 8.3.3.3.5
// FeatureValue): the expression, and whether its operator is `default` or `:=`.
// ok is false for a node that declares no value.
func ValuePartOf(node Node) (value Node, isDefault, isInitial, ok bool) {
	switch n := node.(type) {
	case *Usage:
		value, isDefault, isInitial = n.Value, n.ValueIsDefault, n.ValueIsInitial
	case *SubjectMember:
		value, isDefault, isInitial = n.BindingExpr, n.ValueIsDefault, n.ValueIsInitial
	case *AssumeMember:
		value, isDefault, isInitial = n.Value, n.ValueIsDefault, n.ValueIsInitial
	case *RequireMember:
		value, isDefault, isInitial = n.Value, n.ValueIsDefault, n.ValueIsInitial
	}
	if value == nil {
		return nil, false, false, false
	}
	return value, isDefault, isInitial, true
}

// ExpressionOperand is an operand at the slot the RDF conversion names it by,
// bound to `in` parameter Index of its expression (-1: no parameter binds it).
type ExpressionOperand struct {
	Node  Node
	Slot  string
	Index int
}

// ExpressionOperands are an expression's operands in the order the RDF conversion
// writes them, and its type argument's parameter index (-1 when it has none).
func ExpressionOperands(node Node) (operands []ExpressionOperand, typeArgument int) {
	positional := func(nodes []Node, first int) {
		for i, n := range nodes {
			if n != nil {
				operands = append(operands, ExpressionOperand{Node: n, Slot: fmt.Sprintf("a%d", i), Index: first + i})
			}
		}
	}
	invocation := func(receiver Node, function *QualifiedName, args []Node, named []NamedArg) {
		first := 0
		if receiver != nil && function == nil && ReceiverChain(receiver) != nil {
			// `q.p.f(1)`: the receiver is the chain naming the function, not an operand.
			receiver = nil
		}
		if receiver != nil {
			index := -1
			if function != nil {
				index, first = 0, 1
			}
			operands = append(operands, ExpressionOperand{Node: receiver, Slot: "operand", Index: index})
		}
		positional(args, first)
		first += len(args)
		for i, arg := range named {
			if arg.Value != nil {
				operands = append(operands, ExpressionOperand{Node: arg.Value, Slot: fmt.Sprintf("n%d", i), Index: first + i})
			}
		}
	}
	switch n := node.(type) {
	case *OperatorExpr:
		positional(n.Operands, 0)
		if n.TypeRef != nil {
			return operands, len(n.Operands)
		}
		return operands, -1
	case *CastExpr:
		return nil, 0
	case *FeatureChainExpr:
		positional([]Node{n.Operand}, 0)
	case *IndexExpr:
		positional([]Node{n.Operand, n.Index}, 0)
	case *CollectExpr:
		positional([]Node{n.Operand, n.Body}, 0)
	case *SelectExpr:
		positional([]Node{n.Operand, n.Body}, 0)
	case *SequenceExpr:
		positional(n.Elements, 0)
	case *TimeEvent:
		positional([]Node{n.Duration}, 0)
	case *InvocationExpr:
		invocation(n.Operand, n.Type, n.Args, n.NamedArgs)
	case *ConstructorExpr:
		invocation(nil, n.Type, n.Args, n.NamedArgs)
	}
	return operands, -1
}

// ReceiverChain is the feature chain a call reached through its receiver names
// its function by (`q.p.f(1)`: q, p, f), one single-segment name per link; nil
// when any link is an expression rather than a name.
func ReceiverChain(receiver Node) []*QualifiedName {
	var links []*QualifiedName
	for {
		switch n := receiver.(type) {
		case *FeatureChainExpr:
			if n.Member == nil || QualifiedNameHasChain(n.Member) {
				return nil
			}
			links = append(links, n.Member)
			receiver = n.Operand
			continue
		case *FeatureReference:
			links = append(links, n.Name)
		case *QualifiedName:
			links = append(links, n)
		default:
			return nil
		}
		break
	}
	if len(links) < 2 {
		return nil
	}
	for i, j := 0, len(links)-1; i < j; i, j = i+1, j-1 {
		links[i], links[j] = links[j], links[i]
	}
	return links
}

// QualifiedNameHasChain reports whether name reaches its element through a
// dotted feature chain (`a.b`), which the metamodel owns as a chain feature.
func QualifiedNameHasChain(name *QualifiedName) bool {
	for i := 1; i < len(name.Parts); i++ {
		if name.Parts[i].Chained {
			return true
		}
	}
	return false
}

// BodyDeclaration is one declaration of an expression body (`{ in x; attribute
// y = 2; x + y }`) at the position the RDF conversion numbers it by: Param for a
// parameter, written at slot `in%d`, else Member, written at slot `m%d`.
type BodyDeclaration struct {
	Index  int
	Param  *BodyParam
	Member Node // the member as written, membership wrapper and all
	Decl   Node // the declaration the member holds
}

// Slot is the name the conversion writes the declaration under.
func (d BodyDeclaration) Slot() string {
	if d.Param != nil {
		return fmt.Sprintf("in%d", d.Index)
	}
	return fmt.Sprintf("m%d", d.Index)
}

// BodyDeclarations are the parameters and members of an expression body in the
// order the RDF conversion numbers them: source order across both lists.
func BodyDeclarations(params []BodyParam, members []Node) []BodyDeclaration {
	out := make([]BodyDeclaration, 0, len(params)+len(members))
	for i := range params {
		out = append(out, BodyDeclaration{Param: &params[i]})
	}
	for _, member := range members {
		out = append(out, BodyDeclaration{Member: member, Decl: unwrapMember(member)})
	}
	offset := func(d BodyDeclaration) int {
		if d.Param != nil {
			return d.Param.Span.Offset
		}
		return d.Member.Span().Offset
	}
	sort.SliceStable(out, func(i, j int) bool { return offset(out[i]) < offset(out[j]) })
	for i := range out {
		out[i].Index = i
	}
	return out
}
