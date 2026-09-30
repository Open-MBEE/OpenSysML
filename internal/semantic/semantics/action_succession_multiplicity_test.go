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
