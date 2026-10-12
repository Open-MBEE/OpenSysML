package view

import (
	"strings"
	"testing"
)

// An `include u;` in an exposed use case is a reference to another exposed
// case, drawn by the connection between them, not a node nested in the case.
func TestIncludedUseCaseIsNoNestedNode(t *testing.T) {
	rendering := render(t, "interconnection-includes.sysml", "CaseViews::useCases")
	if got := nodeNames(everyNode(rendering.Roots)); len(got) != 3 {
		t.Errorf("nodes = %v, want the actor and the two cases only", got)
	}
	if len(rendering.Edges) != 2 {
		t.Errorf("edges = %d, want the two connections: %+v", len(rendering.Edges), rendering.Edges)
	}
	for _, notice := range rendering.Notices {
		if strings.Contains(notice, "without a position") || strings.Contains(notice, "does not expose") {
			t.Errorf("notice %q, want none about the include", notice)
		}
	}
}

// A use case's subject and actor parameters are pins on its node, as a case's
// parameters are drawn, not nodes nested in it: a connection to the actor
// parameter ends at the use case node, on the actor's pin.
func TestUseCaseActorParameterIsAPinOnItsNode(t *testing.T) {
	rendering := render(t, "interconnection-actors.sysml", "CaseViews::useCases")
	nodes := everyNode(rendering.Roots)
	if got := nodeNames(nodes); len(got) != 2 {
		t.Errorf("nodes = %v, want the actor and the use case only", got)
	}
	var useCase *Node
	for _, node := range nodes {
		if node.Kind == "use case" {
			useCase = node
		}
	}
	if useCase == nil {
		t.Fatalf("no use case node in %+v", nodes)
	}
	if len(useCase.Children) != 0 {
		t.Errorf("use case has nested nodes %+v, want none", useCase.Children)
	}
	var actorPin string
	for _, port := range useCase.Ports {
		if port.Name == "operator" {
			actorPin = port.ID
		}
	}
	if actorPin == "" {
		t.Fatalf("use case ports = %+v, want the actor parameter among them", useCase.Ports)
	}
	if len(rendering.Edges) != 1 {
		t.Fatalf("edges = %+v, want the one connection", rendering.Edges)
	}
	edge := rendering.Edges[0]
	if edge.To != useCase.ID || edge.ToPort != actorPin {
		t.Errorf("edge ends at %s port %q, want the use case %s pin %q: %+v", edge.To, edge.ToPort, useCase.ID, actorPin, edge)
	}
}
