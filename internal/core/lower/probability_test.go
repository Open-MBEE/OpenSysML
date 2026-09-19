package lower

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// weightedActionGraph lowers action def A of a model importing Stochastic through
// the name-resolution tier, the way the runtime does.
func weightedActionGraph(t *testing.T, body string) (*ActionGraph, error) {
	t.Helper()
	src := "package M {\n import Stochastic::*;\n action def A {\n" + body + "\n }\n}\n"
	p := parser.New(source.New("m.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("m.sysml", root)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot("m.sysml").LookupLocal("M")
	if !ok {
		t.Fatal("package M not indexed")
	}
	a, ok := pkg.Scope.LookupLocal("A")
	if !ok {
		t.Fatal("action def A not indexed")
	}
	return ToActionGraphWith(a.Decl, a.Scope, resolve.New(idx))
}

func TestProbability_ReadOnEverySuccessionForm(t *testing.T) {
	graph, err := weightedActionGraph(t, `
		decide d;
		first d then fast { @Probability { p = 0.7; } }
		succession first d then slow { @Probability { p = 0.2; } }
		first d then other { metadata Probability { p = 0.1; } }
		action fast; action slow; action other;
	`)
	if err != nil {
		t.Fatalf("ToActionGraphWith: %v", err)
	}
	d := namedActionNode(t, graph, "d")
	edges := graph.Edges[d]
	if len(edges) != 3 {
		t.Fatalf("decision has %d edges, want 3", len(edges))
	}
	for i, want := range []float64{0.7, 0.2, 0.1} {
		if edges[i].Probability == nil {
			t.Fatalf("edge %d carries no probability", i)
		}
		got, ok := edges[i].Probability.Constant()
		if !ok || got != want {
			t.Errorf("edge %d weight = %v, %v; want %v", i, got, ok, want)
		}
	}
}

func TestProbability_ArithmeticOverLiteralsIsAConstantWeight(t *testing.T) {
	graph, err := weightedActionGraph(t, `
		decide d;
		first d then fast { @Probability { p = 1 - 0.3; } }
		first d then slow { @Probability { p = 3 / 10; } }
		action fast; action slow;
	`)
	if err != nil {
		t.Fatalf("ToActionGraphWith: %v", err)
	}
	for i, want := range []float64{0.7, 0.3} {
		got, ok := graph.Edges[namedActionNode(t, graph, "d")][i].Probability.Constant()
		if !ok || math.Abs(got-want) > ProbabilityTolerance {
			t.Errorf("edge %d weight = %v, %v; want the arithmetic folded to %v", i, got, ok, want)
		}
	}
}

func TestProbability_NonConstantWeightIsKeptAsExpression(t *testing.T) {
	graph, err := weightedActionGraph(t, `
		attribute w : Real = 0.4;
		decide d;
		first d then fast { @Probability { p = w; } }
		first d then slow { @Probability { p = 1.0 - w; } }
		action fast; action slow;
	`)
	if err != nil {
		t.Fatalf("ToActionGraphWith: %v", err)
	}
	edges := graph.Edges[namedActionNode(t, graph, "d")]
	if _, ok := edges[0].Probability.Constant(); ok {
		t.Error("a weight naming a feature was folded to a constant")
	}
	if _, ok := edges[0].Probability.Expr.(*ast.FeatureReference); !ok {
		t.Errorf("weight expression = %T, want the feature reference as written", edges[0].Probability.Expr)
	}
}

func TestProbability_WithoutResolverReadsNone(t *testing.T) {
	graph := actionGraphFor(t, `
		action a {
			decide d;
			first d then fast { @Probability { p = 0.7; } }
			first d then slow { @Probability { p = 0.3; } }
			action fast; action slow;
		}
	`)
	for _, edge := range graph.Edges[namedActionNode(t, graph, "d")] {
		if edge.Probability != nil {
			t.Fatal("a lowering with no resolver read a Probability annotation")
		}
	}
}

func TestProbability_Refusals(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"mixed", `
			decide d;
			first d then fast { @Probability { p = 0.7; } }
			first d then slow;
			action fast; action slow;`, "weights 1 of its 2 successions"},
		{"sum", `
			decide d;
			first d then fast { @Probability { p = 0.7; } }
			first d then slow { @Probability { p = 0.7; } }
			action fast; action slow;`, "sum to 1.4, not 1.0"},
		{"arithmetic sum", `
			decide d;
			first d then fast { @Probability { p = 0.3 + 0.3; } }
			first d then slow { @Probability { p = 0.2; } }
			action fast; action slow;`, "sum to 0.8, not 1.0"},
		{"range", `
			decide d;
			first d then fast { @Probability { p = 1.5; } }
			first d then slow { @Probability { p = -0.5; } }
			action fast; action slow;`, "p = 1.5 lies outside 0.0..1.0"},
		{"arithmetic range", `
			decide d;
			first d then fast { @Probability { p = 2 * 0.6; } }
			first d then slow { @Probability { p = -0.2; } }
			action fast; action slow;`, "p = 2 * 0.6 lies outside 0.0..1.0"},
		{"not a number", `
			decide d;
			first d then fast { @Probability { p = true; } }
			first d then slow { @Probability { p = 0.5; } }
			action fast; action slow;`, "p = true is not a number"},
		{"not a decision", `
			action fast;
			first fast then slow { @Probability { p = 1.0; } }
			action slow;`, "only a succession out of a decision node can be"},
		{"twice", `
			decide d;
			first d then fast { @Probability { p = 0.5; } @Probability { p = 0.5; } }
			first d then slow { @Probability { p = 0.5; } }
			action fast; action slow;`, "states it twice"},
		{"no p", `
			decide d;
			first d then fast { @Probability {} }
			first d then slow { @Probability { p = 0.5; } }
			action fast; action slow;`, "binds no p"},
		{"other feature", `
			decide d;
			first d then fast { @Probability { q = 0.5; } }
			first d then slow { @Probability { p = 0.5; } }
			action fast; action slow;`, `nothing named "q"`},
		{"stray", `
			decide d;
			@Probability { p = 0.5; } first d then fast;
			@Probability { p = 0.5; } first d then slow;
			action fast; action slow;`, "annotates the action here, not a succession"},
		{"stray in a leaf action", `
			decide d;
			first d then fast; first d then slow;
			action fast { @Probability { p = 1.0; } }
			action slow;`, "annotates the action here, not a succession"},
		{"stray as an action prefix", `
			decide d;
			first d then fast; first d then slow;
			#Probability action fast;
			action slow;`, "annotates the action here, not a succession"},
		{"stray in a control node", `
			decide d { @Probability { p = 1.0; } }
			first d then fast; first d then slow;
			action fast; action slow;`, "annotates the action here, not a succession"},
		{"stray in a start node", `
			first start { @Probability { p = 1.0; } }
			then fast;
			action fast;`, "annotates the action here, not a succession"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := weightedActionGraph(t, tc.body)
			if err == nil {
				t.Fatal("lowering accepted the model")
			}
			if !errors.Is(err, ErrProbability) {
				t.Fatalf("error %v is not an ErrProbability", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}
