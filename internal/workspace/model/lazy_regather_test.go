package model

import (
	"reflect"
	"testing"
)

// TestWorkspacePendingGathersSettleOnRead: an edit queues its gathers instead of
// replaying them, so several edits coalesce into one settle the next read runs —
// and what that read reports is what a fresh analysis of the same text says.
func TestWorkspacePendingGathersSettleOnRead(t *testing.T) {
	const notDerived = "oosem-requirement-not-derived"
	sat := []byte("package S { private import OOSEM::*; #systemRequirement requirement sys; }")

	ws := NewWorkspace()
	ws.Open("hub.sysml", []byte("package M { private import OOSEM::*; #stakeholderNeed requirement need; }"), 1)
	ws.Open("sat.sysml", sat, 1)
	ws.Diagnostics("sat.sysml")

	// Two edits with no read between them queue together: the hub drops its
	// stakeholder need for a mission requirement, the satellite's verdict
	// follows it when the next diagnostics settle the pending regathers.
	ws.Update("hub.sysml", []byte("package M { private import OOSEM::*; #missionRequirement requirement mission; }"), 2)
	ws.Update("sat.sysml", []byte("package S { private import OOSEM::*; #systemRequirement requirement sys; requirement extra; }"), 2)
	gotHub := codesOf(ws.Diagnostics("hub.sysml"))
	gotSat := codesOf(ws.Diagnostics("sat.sysml"))
	if gotSat[notDerived] != 1 {
		t.Fatalf("sat.sysml under a mission requirement added while pending: %v, want one %s", gotSat, notDerived)
	}

	fresh := NewWorkspace()
	fresh.Open("hub.sysml", ws.Document("hub.sysml").Content, 1)
	fresh.Open("sat.sysml", ws.Document("sat.sysml").Content, 1)
	if want := codesOf(fresh.Diagnostics("hub.sysml")); !reflect.DeepEqual(gotHub, want) {
		t.Errorf("hub.sysml: incremental %v, fresh %v", gotHub, want)
	}
	if want := codesOf(fresh.Diagnostics("sat.sysml")); !reflect.DeepEqual(gotSat, want) {
		t.Errorf("sat.sysml: incremental %v, fresh %v", gotSat, want)
	}
}
