package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func TestToActionNodeFlowKeepsInheritedProbabilityWeights(t *testing.T) {
	base := `
		action def Base {
			decide d;
			first start then d;
			first d then fast { @Probability { p = 0.9; } }
			first d then slow { @Probability { p = 0.1; } }
			action fast;
			action slow;
		}
	`
	tests := []struct {
		name, owner, src string
		stateEntry       bool
	}{
		{
			name:  "state entry",
			owner: "m",
			src: `package M {
				import Stochastic::*;
				` + base + `
				state m {
					attribute count : Integer := 0;
					entry action prep : Base {
						assign count := count + 1;
					}
				}
			}`,
			stateEntry: true,
		},
		{
			name:  "classifier perform",
			owner: "Monitor",
			src: `package M {
				import Stochastic::*;
				` + base + `
				part def Monitor {
					attribute count : Integer := 0;
					perform action prep : Base {
						assign count := count + 1;
					}
				}
			}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			graph, node := actionNodeFlowFromSource(t, tc.src, tc.owner, "prep", tc.stateEntry)
			subflow := graph.Subflows[node]
			if subflow == nil || subflow.Graph == nil || subflow.Err != nil {
				t.Fatalf("typed behavior subflow = %#v, want a merged action graph", subflow)
			}
			decision := namedActionNode(t, subflow.Graph, "d")
			edges := subflow.Graph.Edges[decision]
			if len(edges) != 2 {
				t.Fatalf("inherited decision has %d edges, want 2", len(edges))
			}
			for i, want := range []float64{0.9, 0.1} {
				if edges[i].Probability == nil {
					t.Fatalf("inherited edge %d has no probability", i)
				}
				got, ok := edges[i].Probability.Constant()
				if !ok || got != want {
					t.Errorf("inherited edge %d weight = %v, %v; want %v", i, got, ok, want)
				}
			}
		})
	}
}

func TestToActionNodeFlowDefersRecursiveTypedAction(t *testing.T) {
	src := `package M {
		action def A {
			attribute c : Integer := 0;
			first start then x;
			action x : A {
				assign c := c + 1;
			}
			then done;
		}
		state m {
			entry action prep : A {
				assign c := c + 1;
			}
		}
	}`
	graph, entry := actionNodeFlowFromSource(t, src, "m", "prep", true)
	prep := graph.Subflows[entry]
	if prep == nil || prep.Graph == nil || prep.Err != nil {
		t.Fatalf("entry behavior subflow = %#v, want A's graph", prep)
	}
	unfoldedDeferred(t, prep.Graph, "x")
}

func actionNodeFlowFromSource(
	t *testing.T, src, ownerName, behaviorName string, stateEntry bool,
) (*ActionGraph, *ast.Usage) {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("test.sysml", root)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	pkg, ok := idx.DocumentRoot("test.sysml").LookupLocal("M")
	if !ok {
		t.Fatal("package M not indexed")
	}
	owner, ok := pkg.Scope.LookupLocal(ownerName)
	if !ok {
		t.Fatalf("%s not indexed", ownerName)
	}
	var behavior *ast.Usage
	if stateEntry {
		state, ok := owner.Decl.(*ast.Usage)
		if !ok {
			t.Fatalf("%s declaration is %T, want *ast.Usage", ownerName, owner.Decl)
		}
		for _, member := range state.Members {
			entry, ok := unwrapMembership(member).(*ast.EntryMember)
			if !ok {
				continue
			}
			for _, action := range entry.Actions {
				usage, ok := unwrapMembership(action).(*ast.Usage)
				if !ok {
					continue
				}
				name, _ := ast.EffectiveName(usage)
				if name == behaviorName {
					behavior = usage
					break
				}
			}
		}
	} else {
		var members []ast.Node
		switch decl := owner.Decl.(type) {
		case *ast.Definition:
			members = decl.Members
		case *ast.Usage:
			members = decl.Members
		}
		for _, candidate := range ClassifierBehaviorsOf(members) {
			if candidate.Kind == PerformedAction && candidate.Name == behaviorName {
				behavior = candidate.Decl
				break
			}
		}
	}
	if behavior == nil {
		t.Fatalf("behavior %q not found under %s", behaviorName, ownerName)
	}
	return ToActionNodeFlow(behavior, owner.Scope, resolver), behavior
}
