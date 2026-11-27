package resolve_test

import "testing"

// A chain written as the argument of an action node reads its target as a member of its
// operand: the target is no name of the node's scope, and a chain that resolves reports nothing.
func TestNodeArgumentChainTargetIsAMemberOfItsOperand(t *testing.T) {
	got := resolvedWithModel(t, `package P {
	part def S { attribute v; attribute xs[*]; }
	action a {
		in s : S;
		attribute n;
		action aa { out part tgt; out part other; }
		action snd1 send s via this to aa.tgt;
		action snd2 send aa.other via aa.tgt to aa.tgt;
		assign n := s.v;
		for k in s.xs { }
		if s.v == 1 { }
		while s.v == 1 { }
		action t terminate aa.tgt;
	}
}`)
	if len(got) != 0 {
		t.Fatalf("diagnostics = %v, want none", got)
	}
}
