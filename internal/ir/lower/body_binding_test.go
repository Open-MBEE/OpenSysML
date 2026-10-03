package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestLoweredBodyFeaturesPreserveValueOperators checks lowered action bindings and defaults.
func TestLoweredBodyFeaturesPreserveValueOperators(t *testing.T) {
	graph := actionGraphFor(t, `
		action act {
			attribute bound : Integer = 1;
			attribute initial : Integer := 2;
			attribute fallback : Integer default 3;
			action nested {
				attribute localBound : Integer = 4;
				attribute localInitial : Integer := 5;
				attribute localFallback : Integer default 6;
			}
			if true {
				attribute blockBound : Integer = 7;
				attribute blockInitial : Integer := 8;
				attribute blockFallback : Integer default 9;
			}
		}
	`)

	root := make(map[string]bool, len(graph.Attributes))
	for _, attr := range graph.Attributes {
		root[attr.Name] = attr.Binding
	}
	if root["bound"] != true || root["initial"] || root["fallback"] {
		t.Errorf("action attribute bindings = %v", root)
	}

	var nested ast.Node
	for _, node := range graph.Nodes {
		if getNodeName(node) == "nested" {
			nested = node
			break
		}
	}
	features := graph.Features[nested]
	nestedBindings := make(map[string]bool, len(features))
	for _, feature := range features {
		nestedBindings[feature.Name] = feature.Binding
	}
	if nestedBindings["localBound"] != true || nestedBindings["localInitial"] || nestedBindings["localFallback"] {
		t.Errorf("nested action feature bindings = %v", nestedBindings)
	}

	var local Declare
	for _, node := range graph.Nodes {
		for _, statement := range graph.Bodies[node] {
			branch, ok := statement.(If)
			if !ok {
				continue
			}
			for _, nestedStatement := range branch.Then.Statements {
				if declared, ok := nestedStatement.(Declare); ok && declared.Name == "blockBound" {
					local = declared
				}
			}
		}
	}
	if local.Name != "blockBound" || !local.Binding {
		t.Fatalf("block binding declaration = %#v", local)
	}
}

// TestStateGraphPreservesAttributeValueOperators checks state attribute binding flags.
func TestStateGraphPreservesAttributeValueOperators(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		state Machine {
			attribute bound : Integer = 1;
			attribute initial : Integer := 2;
			attribute fallback : Integer default 3;
			entry; then active;
			state active {
				attribute stateBound : Integer = 4;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	root := make(map[string]bool, len(graph.Attributes))
	for _, attr := range graph.Attributes {
		root[attr.Name] = attr.Binding
	}
	if root["bound"] != true || root["initial"] || root["fallback"] {
		t.Errorf("state machine attribute bindings = %v", root)
	}
	for state, attrs := range graph.StateAttributes {
		if state.Name != "active" {
			continue
		}
		if len(attrs) != 1 || attrs[0].Name != "stateBound" || !attrs[0].Binding {
			t.Errorf("state attributes = %#v, want a binding stateBound", attrs)
		}
		return
	}
	t.Fatal("state attributes for active were not lowered")
}
