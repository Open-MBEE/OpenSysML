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
