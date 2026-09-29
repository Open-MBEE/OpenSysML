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
