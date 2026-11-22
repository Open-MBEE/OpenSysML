package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessTimeEventPayload: a time event fires only the lowered
// transition it carries; a syntax edge is refused, never lowered at dispatch.
func TestRuntimeRobustnessTimeEventPayload(t *testing.T) {
	t.Run("syntax_edge_is_refused", testTimeEventSyntaxEdgeIsRefused)
	t.Run("syntax_edge_is_not_a_completion_event", testSyntaxEdgeIsNotACompletionEvent)
}

func testTimeEventSyntaxEdgeIsRefused(t *testing.T) {
	exec := stateExecutorForSource(t, "M", `package P {
		state def M {
			entry; then a;
			state a;
			state b;
		}
	}`)
	edge := &ast.TransitionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "a"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "b"}}},
	}
	before := exec.getCurrentState()
	dispatch, err := exec.dispatchEvent(Event{ID: 1, Type: eventTime, Payload: edge})
	if err == nil || !strings.Contains(err.Error(), "*ast.TransitionEdge") {
		t.Fatalf("dispatchEvent error = %v, want the syntax edge refused by type", err)
	}
	if dispatch.Fired || exec.getCurrentState() != before {
		t.Fatalf("fired = %v, state = %v: a refused payload must move nothing", dispatch.Fired, exec.getCurrentState())
	}
}

func testSyntaxEdgeIsNotACompletionEvent(t *testing.T) {
	if isCompletionEvent(Event{Type: eventTime, Payload: &ast.TransitionEdge{}}) {
		t.Fatal("a syntax edge was read as a completion transition")
	}
}
