package model

import (
	"strings"
	"testing"
)

// Re-opening a document whose transition assigns its trigger parameter to a
// feature of the same type keeps the binding check clean, as it was at first.
func TestTriggerParameterAssignmentSurvivesReopen(t *testing.T) {
	const model = `package Demo {
	state def SD {
		item s32 : s3;
		entry; then a;
		state a;
		state b;
		transition first a accept s3 : s3 do action { assign s32 := s3; } then b;
	}
	item def s3;
}`
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte(model), 1)
	if d := ws.Diagnostics("a.sysml"); len(d) != 0 {
		t.Fatalf("diagnostics = %d, want 0: %+v", len(d), d)
	}
	ws.Open("a.sysml", []byte(model+"\npackage Other { part def Q; }\n"), 2)
	for _, d := range ws.Diagnostics("a.sysml") {
		if strings.Contains(d.Message, "cannot bind") {
			t.Fatalf("reopen reports %q", d.Message)
		}
	}
}
