package pssm

import (
	"strings"
	"testing"
)

// TestValidateMetadataStateMachine: a machine spelled with the StateMachines
// metadata the migrator emits validates and lowers cleanly — the lowering runs
// through the workspace resolver, or every pseudostate the annotations declare
// is silently dropped from the graph.
func TestValidateMetadataStateMachine(t *testing.T) {
	m := &Model{
		Name:      "MetadataMachine.sysml",
		Qualified: "MetadataMachine::M",
		Text: `package MetadataMachine {
	private import StateMachines::*;
	item def Go;
	state M {
		entry; then idle;
		state idle;
		state a;
		transition first idle accept Go then pick;
		#StateMachines::choice state pick;
		transition first pick if true then busy;
		state busy;
	}
}
`,
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}
}
