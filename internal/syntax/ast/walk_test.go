package ast

import "testing"

func bitnot(operand Node) *OperatorExpr {
	return &OperatorExpr{Operator: OpBitNot, Operands: []Node{operand}}
}

// Inspect reaches a `~` wherever the tree carries an expression: a feature
// value, a relationship target, a multiplicity bound, a transition guard and
// the members of a nested body, and it reports a shared node once.
func TestInspectVisitsEveryOperator(t *testing.T) {
	shared := bitnot(&LiteralInteger{Value: "9"})
	root := &RootNamespace{Members: []Node{
		&Usage{Keyword: "feature", Value: bitnot(&LiteralInteger{Value: "5"})},
		&Usage{Keyword: "feature", Multiplicity: &Multiplicity{
			IsRange: true, Lower: bitnot(&LiteralInteger{Value: "1"}), Upper: &LiteralInteger{Value: "2"},
		}},
		&Package{Members: []Node{
			&TransitionMember{Guard: bitnot(&QualifiedName{Parts: []NameSegment{{Text: "x"}}})},
			&Relationship{Target: shared},
			&Membership{Member: shared},
			&Usage{Keyword: "feature", Value: &OperatorExpr{Operator: OpAdd, Operands: []Node{
				&LiteralInteger{Value: "1"}, bitnot(&LiteralInteger{Value: "2"}),
			}}},
		}},
	}}
	var ops []*OperatorExpr
	visits := map[Node]int{}
	Inspect(root, func(n Node) bool {
		visits[n]++
		if op, ok := n.(*OperatorExpr); ok && op.Operator == OpBitNot {
			ops = append(ops, op)
		}
		return true
	})
	if len(ops) != 5 {
		t.Errorf("visited %d OpBitNot expressions, want 5", len(ops))
	}
	for n, count := range visits {
		if count != 1 {
			t.Errorf("%T visited %d times, want 1", n, count)
		}
	}
}

// visit answering false prunes the descent, nil trees are safe, and a cycle
// ends the walk.
func TestInspectPrunesAndTerminates(t *testing.T) {
	inner := bitnot(&LiteralInteger{Value: "1"})
	root := &RootNamespace{Members: []Node{
		&Usage{Keyword: "feature", Value: &OperatorExpr{Operator: OpAdd, Operands: []Node{
			inner, &LiteralInteger{Value: "2"},
		}}},
	}}
	var seen []Node
	Inspect(root, func(n Node) bool {
		seen = append(seen, n)
		return n != root.Members[0] // prune at the usage
	})
	for _, n := range seen {
		if n == inner {
			t.Errorf("pruned node was visited")
		}
	}
	Inspect(nil, func(Node) bool { return true })
	var nilUsage *Usage
	Inspect(nilUsage, func(Node) bool { return true })
	loop := &Package{}
	loop.Members = []Node{loop}
	count := 0
	Inspect(loop, func(Node) bool { count++; return true })
	if count != 1 {
		t.Errorf("cyclic package visited %d times, want 1", count)
	}
}
