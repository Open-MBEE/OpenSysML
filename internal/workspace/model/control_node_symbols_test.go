package model

import "testing"

// TestSuccessionNamesControlNode covers a succession naming a control node as
// source or target: each control node carries a name the symbol table records.
func TestSuccessionNamesControlNode(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"fork", `package P { action def F { first start; fork Jump; action S; first start then Jump; first Jump then S; } }`},
		{"join", `package P { action def F { first start; join Land; action S; first start then S; first S then Land; } }`},
		{"merge", `package P { action def F { first start; merge M; action S; first start then S; first S then M; } }`},
		{"decision", `package P { action def F { first start; decide D; action S; first start then S; first S then D; } }`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := diagnose(t, "control_"+tt.name, tt.src); len(got) > 0 {
				t.Errorf("unexpected findings: %v", got)
			}
		})
	}
}

// TestUnnamedControlNodeStillResolves covers unnamed control nodes: they
// declare no name, so they register none and the body still resolves.
func TestUnnamedControlNodeStillResolves(t *testing.T) {
	src := `package P { action def F { first start; fork; join; merge; decide; action S; first start then S; } }`
	if got := diagnose(t, "control_unnamed", src); len(got) > 0 {
		t.Errorf("unexpected findings: %v", got)
	}
}

// TestChainedBindingEndDeclaresNoMember covers a chained binding end: its last
// segment names an existing feature, so the binding declares no member for it.
func TestChainedBindingEndDeclaresNoMember(t *testing.T) {
	src := `package P {
		part R { part inner { attribute h; } }
		part c;
		binding bind R.inner.h = c;
		part def Uses { attribute x = h; }
	}`
	got := diagnose(t, "binding_chain_member", src)
	if len(got) != 1 {
		t.Fatalf("want one unresolved finding for h, got %v", got)
	}
}
