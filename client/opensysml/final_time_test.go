package opensysml_test

import (
	"context"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const timedSource = `package Timed {
	private import ScalarValues::*;

	action waiter {
		out fired : Boolean = false;
		first start;
		then action wait accept after 5 [SI::s];
		then action fire assign fired := true;
		then done;
	}

	state Timer {
		attribute fired : Integer = 0;
		entry; then armed;
		state armed;
		transition armed then done accept after 3 [SI::s] do assign fired := fired + 1;
	}
}`

// A run reports the clock it ended at: the instant its last timed wait came due.
func TestARunReportsItsFinalTime(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	info, err := client.ServerInfo(ctx)
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if !info.Has(opensysml.CapabilityFinalTime) {
		t.Fatalf("capabilities = %v, want it to contain %q", info.Capabilities, opensysml.CapabilityFinalTime)
	}
	model := parse(t, client, timedSource)

	action, err := client.ExecuteAction(ctx, model, "Timed::waiter", nil)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if action.FinalTime != 5 {
		t.Errorf("ExecuteAction: FinalTime = %v, want 5", action.FinalTime)
	}
	if got := action.Outputs["fired"]; got != opensysml.Bool(true) {
		t.Errorf("ExecuteAction: fired = %#v, want Bool(true)", got)
	}

	state, err := client.ExecuteState(ctx, model, "Timed::Timer", nil)
	if err != nil {
		t.Fatalf("ExecuteState: %v", err)
	}
	if state.FinalTime != 3 {
		t.Errorf("ExecuteState: FinalTime = %v, want 3", state.FinalTime)
	}
	if got := state.Context["fired"]; got != opensysml.Int(1) {
		t.Errorf("ExecuteState: fired = %#v, want Int(1)", got)
	}
}
