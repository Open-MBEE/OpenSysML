package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestActionSuccessionSourceEndMultiplicity(t *testing.T) {
	m, root := buildModel(t, "action def A { action a; [2] then b; action b; }")
	var edge *ast.SuccessionEdge
	for _, succession := range m.ActionSuccessions(sym(t, root, "A")) {
		if candidate, ok := succession.Decl.(*ast.SuccessionEdge); ok && candidate.SourceMultiplicity != nil {
			edge = candidate
			if succession.Source.Multiplicity != candidate.SourceMultiplicity {
				t.Fatal("semantic source end did not retain the parsed multiplicity")
			}
		}
	}
	if edge == nil {
		t.Fatal("source-end multiplicity edge not found")
	}
}

// `then [m] b;` writes the target end's crossing multiplicity, which the
// semantic target end retains while the source end carries none.
func TestActionSuccessionTargetEndMultiplicity(t *testing.T) {
	m, root := buildModel(t, "action def A { action a; action b; then [0..1] b; [1] then [2] done; }")
	var edges int
	for _, succession := range m.ActionSuccessions(sym(t, root, "A")) {
		edge, ok := succession.Decl.(*ast.SuccessionEdge)
		if !ok || edge.TargetMultiplicity == nil {
			continue
		}
		edges++
		if succession.Target.Multiplicity != edge.TargetMultiplicity {
			t.Error("semantic target end did not retain the parsed multiplicity")
		}
		if succession.Source.Multiplicity != edge.SourceMultiplicity {
			t.Error("semantic source end does not match the parsed source multiplicity")
		}
	}
	if edges != 2 {
		t.Fatalf("found %d target-end multiplicity edges, want 2", edges)
	}
}

// A guarded succession's `then [m] b` writes the same target end: on the
// transition a `succession first a if g then [m] b` declares, and on the
// `first a if g then [m] b` shorthand.
func TestGuardedSuccessionTargetEndMultiplicity(t *testing.T) {
	m, root := buildModel(t, `action def A {
		action a; action b; action c;
		succession first a if true then [0..1] b;
		first a if true then [*] c;
	}`)
	var transition, initial int
	for _, succession := range m.ActionSuccessions(sym(t, root, "A")) {
		switch decl := succession.Decl.(type) {
		case *ast.TransitionMember:
			if decl.TargetMultiplicity == nil {
				continue
			}
			transition++
			if succession.Target.Multiplicity != decl.TargetMultiplicity {
				t.Error("guarded succession's semantic target end did not retain the parsed multiplicity")
			}
		case *ast.InitialNode:
			if decl.TargetMultiplicity == nil {
				continue
			}
			initial++
			if succession.Target.Multiplicity != decl.TargetMultiplicity {
				t.Error("guarded first's semantic target end did not retain the parsed multiplicity")
			}
		}
	}
	if transition != 1 || initial != 1 {
		t.Fatalf("found %d guarded successions and %d guarded firsts with a target end, want 1 each", transition, initial)
	}
}

// `then [m] fork;` after `action fork;` reaches the declared member by name and
// carries the multiplicity on that target end; no fork node is declared.
func TestActionSuccessionTargetMultiplicityReachesADeclaredNodeWordMember(t *testing.T) {
	m, root := buildModel(t, "action def A { action a; action fork; then [0..1] fork; }")
	fork := sym(t, sym(t, root, "A").Scope, "fork")
	var found bool
	for _, succession := range m.ActionSuccessions(sym(t, root, "A")) {
		edge, ok := succession.Decl.(*ast.SuccessionEdge)
		if !ok || edge.TargetMultiplicity == nil {
			continue
		}
		found = true
		if succession.Target.Symbol != fork {
			t.Errorf("target symbol = %v, want the declared action fork", succession.Target.Symbol)
		}
		if succession.Target.Multiplicity != edge.TargetMultiplicity {
			t.Error("semantic target end did not retain the parsed multiplicity")
		}
	}
	if !found {
		t.Fatal("no succession with a target multiplicity")
	}
}
