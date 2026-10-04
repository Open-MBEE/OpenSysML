package lower

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func TestActionGraphStepCountRequiresExactFiniteMultiplicity(t *testing.T) {
	tests := []struct {
		name         string
		multiplicity *ast.Multiplicity
		want         int64
		wantError    bool
	}{
		{name: "undeclared", want: 1},
		{name: "single", multiplicity: stepTestMultiplicity("3", "", false), want: 3},
		{name: "range", multiplicity: stepTestMultiplicity("3", "3", true), want: 3},
		{name: "zero", multiplicity: stepTestMultiplicity("0", "", false), want: 0},
		{name: "unbounded", multiplicity: stepTestMultiplicity("0", "*", true), wantError: true},
		{name: "open", multiplicity: stepTestMultiplicity("2", "5", true), wantError: true},
		{name: "negative", multiplicity: stepTestMultiplicity("-1", "", false), wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := stepTestNode("a")
			graph := &ActionGraph{Multiplicities: map[ast.Node]*ast.Multiplicity{}}
			if test.multiplicity != nil {
				graph.Multiplicities[node] = test.multiplicity
			}
			got, err := graph.StepCount(node, nil)
			if test.wantError {
				var stepErr *StepMultiplicityError
				if !errors.As(err, &stepErr) || stepErr.Code != StepMultiplicityNotFixedCode {
					t.Fatalf("StepCount error = %v, want %s", err, StepMultiplicityNotFixedCode)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("StepCount = %d, %v, want %d, nil", got, err, test.want)
			}
		})
	}
}

func TestActionGraphRejectsUnaddressableStepAndSuccessionBounds(t *testing.T) {
	tests := []struct {
		name            string
		model           string
		step            string
		successionBound bool
	}{
		{
			name: "single bound",
			model: `action def A {
				attribute n = 2**70;
				first start then a;
				action a[n];
				then done;
			}`,
			step: "a",
		},
		{
			name: "equal range",
			model: `action def A {
				attribute n = 2**70;
				first start then a;
				action a[n..n];
				then done;
			}`,
			step: "a",
		},
		{
			name: "upper range bound",
			model: `action def A {
				attribute n = 2**70;
				first start then a;
				action a[1..n];
				then done;
			}`,
			step: "a",
		},
		{
			name: "named bound",
			model: `action def A {
				attribute n = 2**70;
				first start then a;
				action a[n];
				then done;
			}`,
			step: "a",
		},
		{
			name: "succession end",
			model: `action def A {
				attribute n = 2**70;
				action p;
				action a[3];
				succession first [1] p then [n] a;
			}`,
			step:            "a",
			successionBound: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			graph, model := lowerStepMultiplicityModel(t, test.model)
			var node ast.Node
			for candidate := range graph.Multiplicities {
				if getNodeName(candidate) == test.step {
					node = candidate
					break
				}
			}
			if node == nil {
				t.Fatalf("step %q not found in graph", test.step)
			}
			var err error
			if test.successionBound {
				err = graph.CheckStep(node, model)
			} else {
				_, err = graph.StepCount(node, model)
			}
			var stepErr *StepMultiplicityError
			if !errors.As(err, &stepErr) || stepErr.Code != StepMultiplicityUnsupportedCode {
				t.Fatalf("multiplicity error = %v, want %s", err, StepMultiplicityUnsupportedCode)
			}
			if !errors.Is(err, semantics.ErrIntegerUnaddressable) {
				t.Errorf("multiplicity error = %v, want ErrIntegerUnaddressable", err)
			}
			if !strings.Contains(err.Error(), "1180591620717411303424") {
				t.Errorf("multiplicity error = %q, want the exact bound", err)
			}
		})
	}
}

func lowerStepMultiplicityModel(t *testing.T, text string) (*ActionGraph, *semantics.Model) {
	t.Helper()
	p := parser.New(source.New("<test>", []byte("package test {\n"+text+"\n}")))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %+v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", file)
	idx.ExpandWildcardImports()
	matches := idx.LookupQualified("test::A")
	if len(matches) != 1 {
		t.Fatalf("test::A matched %d symbols, want one", len(matches))
	}
	graph, err := ToActionGraph(matches[0].Decl, matches[0].Scope)
	if err != nil {
		t.Fatalf("lower test::A: %v", err)
	}
	return graph, semantics.NewModel(resolve.New(idx))
}

func TestActionGraphMultiplicityText(t *testing.T) {
	node := stepTestNode("a")
	graph := &ActionGraph{
		Multiplicities: map[ast.Node]*ast.Multiplicity{
			node: stepTestMultiplicity("2", "", false),
		},
	}
	if got := graph.MultiplicityText(node, nil); got != "[2]" {
		t.Fatalf("MultiplicityText = %q, want [2]", got)
	}
	if got := graph.MultiplicityText(stepTestNode("b"), nil); got != "" {
		t.Fatalf("MultiplicityText without a declaration = %q, want empty", got)
	}
}

func TestActionGraphCheckStepSuccessions(t *testing.T) {
	tests := []struct {
		name               string
		stepCount          int64
		repeatedIsSource   bool
		neighborCount      int64
		sourceMultiplicity *ast.Multiplicity
		targetMultiplicity *ast.Multiplicity
		guard              ast.Node
		control            bool
		wantCode           string
	}{
		{
			name:               "source and target ends constrain exact counts",
			stepCount:          3,
			neighborCount:      1,
			sourceMultiplicity: stepTestMultiplicity("1", "", false),
			targetMultiplicity: stepTestMultiplicity("0", "*", true),
		},
		{
			name:               "target end constrains exact count",
			stepCount:          3,
			neighborCount:      1,
			targetMultiplicity: stepTestMultiplicity("3", "", false),
		},
		{
			name:               "source wildcard with exact target",
			stepCount:          3,
			repeatedIsSource:   true,
			neighborCount:      1,
			sourceMultiplicity: stepTestMultiplicity("0", "*", true),
			targetMultiplicity: stepTestMultiplicity("1", "", false),
		},
		{
			name:               "both exact end counts",
			stepCount:          2,
			repeatedIsSource:   true,
			neighborCount:      3,
			sourceMultiplicity: stepTestMultiplicity("2", "", false),
			targetMultiplicity: stepTestMultiplicity("3", "", false),
		},
		{
			name:               "written unconstrained ends leave order open",
			stepCount:          3,
			sourceMultiplicity: stepTestMultiplicity("0", "*", true),
			targetMultiplicity: stepTestMultiplicity("0", "*", true),
			wantCode:           StepOrderOpenCode,
		},
		{name: "exact-one reading excludes an unwritten-end count", stepCount: 3, wantCode: StepOrderUnsatisfiableCode},
		{
			name:               "written target excludes repeated count",
			stepCount:          3,
			targetMultiplicity: stepTestMultiplicity("1", "", false),
			wantCode:           StepOrderUnsatisfiableCode,
		},
		{
			name:               "written ends exclude repeated count",
			stepCount:          3,
			sourceMultiplicity: stepTestMultiplicity("1", "", false),
			targetMultiplicity: stepTestMultiplicity("1", "", false),
			wantCode:           StepOrderUnsatisfiableCode,
		},
		{
			name:               "written ends exclude single-count step",
			stepCount:          1,
			sourceMultiplicity: stepTestMultiplicity("2", "", false),
			targetMultiplicity: stepTestMultiplicity("1", "", false),
			wantCode:           StepOrderUnsatisfiableCode,
		},
		{
			name:               "the exact-one default excludes the source count",
			stepCount:          3,
			repeatedIsSource:   true,
			neighborCount:      1,
			targetMultiplicity: stepTestMultiplicity("1", "", false),
			wantCode:           StepOrderUnsatisfiableCode,
		},
		{name: "guarded edge is unsupported", stepCount: 3, guard: &ast.LiteralBool{Value: true}, wantCode: StepMultiplicityUnsupportedCode},
		{name: "control node adjacency is unsupported", stepCount: 3, repeatedIsSource: true, control: true, wantCode: StepMultiplicityUnsupportedCode},
		{name: "guarded edge at single count is unchanged", stepCount: 1, guard: &ast.LiteralBool{Value: true}},
		{name: "control adjacency at single count is unchanged", stepCount: 1, repeatedIsSource: true, control: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start := &ast.InitialNode{}
			p := stepTestNode("p")
			a := stepTestNode("a")
			var other ast.Node = ast.Node(stepTestNode("q"))
			if test.control {
				other = &ast.ForkNode{Name: "fork"}
			}
			done := &ast.FinalNode{}
			graph := &ActionGraph{
				Nodes:   []ast.Node{start, p, a, other, done},
				Initial: start,
				Edges: map[ast.Node][]ActionEdge{
					start: {{Source: start, Target: p}},
					p:     {{Source: p, Target: a}},
					a:     {{Source: a, Target: done}},
				},
				Multiplicities: map[ast.Node]*ast.Multiplicity{
					a: stepTestMultiplicity("3", "", false),
				},
			}
			if test.repeatedIsSource {
				graph.Edges[start] = []ActionEdge{{Source: start, Target: a}}
				graph.Edges[p] = nil
				graph.Edges[a] = []ActionEdge{{
					Source:             a,
					Target:             other,
					Guard:              test.guard,
					SourceMultiplicity: test.sourceMultiplicity,
					TargetMultiplicity: test.targetMultiplicity,
				}}
			} else {
				graph.Edges[p] = []ActionEdge{{
					Source:             p,
					Target:             a,
					Guard:              test.guard,
					SourceMultiplicity: test.sourceMultiplicity,
					TargetMultiplicity: test.targetMultiplicity,
				}}
			}
			if test.stepCount != 0 {
				graph.Multiplicities[a] = stepTestMultiplicity(fmt.Sprint(test.stepCount), "", false)
			}
			if test.neighborCount > 1 {
				graph.Multiplicities[other] = stepTestMultiplicity(fmt.Sprint(test.neighborCount), "", false)
			}
			if err := graph.CheckStep(a, nil); test.wantCode == "" {
				if err != nil {
					t.Fatalf("CheckStep error = %v, want nil", err)
				}
			} else {
				var stepErr *StepMultiplicityError
				if !errors.As(err, &stepErr) || stepErr.Code != test.wantCode {
					t.Fatalf("CheckStep error = %v, want code %q", err, test.wantCode)
				}
			}
		})
	}
}

func TestActionGraphCheckStepRejectsRepeatedPins(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*ActionGraph, ast.Node)
	}{
		{
			name: "object flow",
			configure: func(graph *ActionGraph, node ast.Node) {
				graph.DataFlows = map[ast.Node][]ObjectFlow{
					node: {{SourcePin: "out", TargetPin: "in"}},
				}
			},
		},
		{
			name: "nested object flow path",
			configure: func(graph *ActionGraph, _ ast.Node) {
				outer := stepTestNode("outer")
				graph.EnclosingNode = outer
				graph.Enclosing = &ActionGraph{
					DataFlows: map[ast.Node][]ObjectFlow{
						outer: {{SourcePin: "a.out", Target: stepTestNode("q"), TargetPin: "in"}},
					},
				}
			},
		},
		{
			name: "binding",
			configure: func(graph *ActionGraph, node ast.Node) {
				graph.Bindings = []PinBinding{{Node: node, Pin: "in"}}
			},
		},
		{
			name: "nested binding path",
			configure: func(graph *ActionGraph, node ast.Node) {
				outer := stepTestNode("outer")
				graph.Enclosing = &ActionGraph{
					Bindings: []PinBinding{{Node: outer, Path: []ast.Node{node}, Pin: "in"}},
				}
				graph.EnclosingNode = outer
			},
		},
		{
			name: "connection",
			configure: func(graph *ActionGraph, _ ast.Node) {
				graph.Connections = []Connection{{Ends: []string{"a.out", "q.in"}}}
			},
		},
		{
			name: "nested connection path",
			configure: func(graph *ActionGraph, _ ast.Node) {
				outer := stepTestNode("outer")
				graph.Enclosing = &ActionGraph{
					Connections: []Connection{{Ends: []string{"outer.a.out", "q.in"}}},
				}
				graph.EnclosingNode = outer
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start := &ast.InitialNode{}
			a := stepTestNode("a")
			done := &ast.FinalNode{}
			graph := &ActionGraph{
				Nodes:   []ast.Node{start, a, done},
				Initial: start,
				Edges: map[ast.Node][]ActionEdge{
					start: {{Source: start, Target: a}},
					a:     {{Source: a, Target: done}},
				},
				Multiplicities: map[ast.Node]*ast.Multiplicity{
					a: stepTestMultiplicity("3", "", false),
				},
			}
			test.configure(graph, a)
			var stepErr *StepMultiplicityError
			if err := graph.CheckStep(a, nil); !errors.As(err, &stepErr) ||
				stepErr.Code != StepMultiplicityUnsupportedCode {
				t.Fatalf("CheckStep error = %v, want %s", err, StepMultiplicityUnsupportedCode)
			}
		})
	}
}

func TestActionGraphCheckStepZeroCountOrdering(t *testing.T) {
	start := &ast.InitialNode{}
	p := stepTestNode("p")
	zero := stepTestNode("zero")
	q := stepTestNode("q")
	graph := &ActionGraph{
		Nodes:   []ast.Node{start, p, zero, q},
		Initial: start,
		Edges: map[ast.Node][]ActionEdge{
			start: {{Source: start, Target: p}},
			p:     {{Source: p, Target: zero}},
			zero:  {{Source: zero, Target: q}},
		},
		Multiplicities: map[ast.Node]*ast.Multiplicity{
			zero: stepTestMultiplicity("0", "", false),
		},
	}
	var stepErr *StepMultiplicityError
	if err := graph.CheckStep(zero, nil); !errors.As(err, &stepErr) || stepErr.Code != StepOrderOpenCode {
		t.Fatalf("CheckStep error = %v, want %s", err, StepOrderOpenCode)
	}

	startGraph := &ActionGraph{
		Nodes:   []ast.Node{start, zero, q},
		Initial: start,
		Edges: map[ast.Node][]ActionEdge{
			start: {{Source: start, Target: zero}},
			zero:  {{Source: zero, Target: q}},
		},
		Multiplicities: map[ast.Node]*ast.Multiplicity{
			zero: stepTestMultiplicity("0", "", false),
		},
	}
	if err := startGraph.CheckStep(zero, nil); err != nil {
		t.Fatalf("zero step at flow start: CheckStep error = %v, want nil", err)
	}

	p = stepTestNode("p")
	zero = stepTestNode("zero")
	q = stepTestNode("q")
	actionStartGraph := &ActionGraph{
		Nodes:   []ast.Node{p, zero, q},
		Initial: p,
		Edges: map[ast.Node][]ActionEdge{
			p:    {{Source: p, Target: zero}},
			zero: {{Source: zero, Target: q}},
		},
		Multiplicities: map[ast.Node]*ast.Multiplicity{
			zero: stepTestMultiplicity("0", "", false),
		},
	}
	if err := actionStartGraph.CheckStep(zero, nil); !errors.As(err, &stepErr) || stepErr.Code != StepOrderOpenCode {
		t.Fatalf("zero step after initial action: CheckStep error = %v, want %s", err, StepOrderOpenCode)
	}
}

func TestActionGraphCheckStepChecksInitialActionPredecessor(t *testing.T) {
	p := stepTestNode("p")
	a := stepTestNode("a")
	graph := &ActionGraph{
		Nodes:   []ast.Node{p, a},
		Initial: p,
		Edges: map[ast.Node][]ActionEdge{
			p: {{Source: p, Target: a}},
		},
		Multiplicities: map[ast.Node]*ast.Multiplicity{
			a: stepTestMultiplicity("3", "", false),
		},
	}
	var stepErr *StepMultiplicityError
	if err := graph.CheckStep(a, nil); !errors.As(err, &stepErr) || stepErr.Code != StepOrderUnsatisfiableCode {
		t.Fatalf("CheckStep error = %v, want %s", err, StepOrderUnsatisfiableCode)
	}
}

func stepTestNode(name string) *ast.Usage {
	return &ast.Usage{Ident: ast.Identification{ShortName: name}}
}

func stepTestMultiplicity(lower, upper string, isRange bool) *ast.Multiplicity {
	multiplicity := &ast.Multiplicity{
		Lower:   &ast.LiteralInteger{Value: lower},
		IsRange: isRange,
	}
	if lower == "*" {
		multiplicity.Lower = &ast.LiteralInfinity{}
	}
	if isRange {
		if upper == "*" {
			multiplicity.Upper = &ast.LiteralInfinity{}
		} else {
			multiplicity.Upper = &ast.LiteralInteger{Value: upper}
		}
	}
	return multiplicity
}
