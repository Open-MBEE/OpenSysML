package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A directed concern, requirement, use case or other usage kind a behavior body
// admits is a parameter of the owning action, as a directed item or part is.
func TestActionGraphDirectedUsageKindsAreParameters(t *testing.T) {
	graph := actionGraphFor(t, `
		action elicit {
			in concern problem : C;
			out requirement spec : R { subject s : I; }
			out use case uses[*] : U;
			out verification checks[*] : V;
			inout item product : I;
			concern aside : C;
		}
	`)
	want := []struct {
		name      string
		kind      ast.UsageKind
		direction ast.FeatureDirection
	}{
		{"problem", ast.UsageConcern, ast.DirIn},
		{"spec", ast.UsageRequirement, ast.DirOut},
		{"uses", ast.UsageUseCase, ast.DirOut},
		{"checks", ast.UsageVerificationCase, ast.DirOut},
		{"product", ast.UsageItem, ast.DirInOut},
	}
	if len(graph.Parameters) != len(want) {
		t.Fatalf("parameters = %+v, want %d", graph.Parameters, len(want))
	}
	for i, w := range want {
		got := graph.Parameters[i]
		u, _ := got.Node.(*ast.Usage)
		if got.Name != w.name || got.Direction != w.direction || u == nil || u.Kind != w.kind {
			t.Errorf("parameter %d = %+v, want %s %v/%v", i, got, w.name, w.kind, w.direction)
		}
	}
}

// A directed analysis or verification usage is a parameter of the action, not a
// case the action performs as a step: only the undirected one is a node.
func TestDirectedCaseUsagesAreNotSteps(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			in analysis study : S;
			out verification checks[*] : V;
			verification inline : V;
			action work;
		}
	`)
	var nodes []string
	for _, n := range graph.Nodes {
		if u, ok := n.(*ast.Usage); ok {
			nodes = append(nodes, u.Ident.Name)
		}
	}
	if len(nodes) != 2 || nodes[0] != "inline" || nodes[1] != "work" {
		t.Errorf("nodes = %v, want [inline work]", nodes)
	}
	if len(graph.Parameters) != 2 || graph.Parameters[0].Name != "study" || graph.Parameters[1].Name != "checks" {
		t.Errorf("parameters = %+v, want study, checks", graph.Parameters)
	}
	for _, n := range graph.Nodes {
		if u, ok := n.(*ast.Usage); ok && IsCaseNode(u) && u.Direction != ast.DirNone {
			t.Errorf("directed case %s is a node", u.Ident.Name)
		}
	}
}

// A use case usage nested in a body is a step of it; a directed one is a
// parameter of the action alone, like a directed analysis or verification usage.
func TestDirectedUseCaseUsageIsNotAStep(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			out use case uses[*] : U;
			use case inline : U;
			action work;
		}
	`)
	var nodes []string
	for _, n := range graph.Nodes {
		if u, ok := n.(*ast.Usage); ok {
			nodes = append(nodes, u.Ident.Name)
		}
	}
	if len(nodes) != 2 || nodes[0] != "inline" || nodes[1] != "work" {
		t.Errorf("nodes = %v, want [inline work]", nodes)
	}
	if len(graph.Parameters) != 1 || graph.Parameters[0].Name != "uses" {
		t.Errorf("parameters = %+v, want uses", graph.Parameters)
	}
}
