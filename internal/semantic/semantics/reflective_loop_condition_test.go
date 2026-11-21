package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestReflectiveLoopUntilOwnsItsUntilCondition(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    action def Poll {
        attribute done : ScalarValues::Boolean;
        loop { action step; } until done;
        while not done { action again; } until done;
    }
}`)
	poll := nestedSym(t, root, "P::Poll")
	var slots [][]string
	for _, membership := range m.OwnedMemberships(poll) {
		member := membership.Implicit.Member
		if member == nil {
			continue
		}
		if _, ok := member.Decl.(*ast.WhileLoopActionNode); !ok {
			continue
		}
		var own []string
		for _, inner := range m.OwnedMemberships(member) {
			if part := inner.Implicit.Member; part != nil && part.Expr != nil && part.Expr.Role == symbols.ExpressionOperand {
				own = append(own, part.Expr.Slot)
			}
		}
		slots = append(slots, own)
	}
	if len(slots) != 2 {
		t.Fatalf("Poll owns %d loops, want 2", len(slots))
	}
	if len(slots[0]) != 1 || slots[0][0] != "untilCondition" {
		t.Errorf("`loop … until` owns the conditions %v, want [untilCondition]", slots[0])
	}
	if len(slots[1]) != 2 || slots[1][0] != "whileCondition" || slots[1][1] != "untilCondition" {
		t.Errorf("`while … until` owns the conditions %v, want [whileCondition untilCondition]", slots[1])
	}
}
