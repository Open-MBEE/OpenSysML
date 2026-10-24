package opensysml_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const lintedSource = `package Linted {
	private import ScalarValues::*;
	attribute x : Real = 0.1;
	state def M { entry; then a; state a; state b; transition first a when Pnig then b; }
}`

// WithAdvance runs for a simulated duration and reports the clock it stopped at,
// on the failure of a run still waiting as on a completed one; WithClockStep
// rounds a wait up to the next tick.
func TestRunOptionsAdvanceAndClockStep(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, timedSource)

	_, err := client.ExecuteAction(ctx, model, "Timed::waiter", nil, opensysml.WithAdvance(2))
	var failure *opensysml.FailureError
	if !errors.As(err, &failure) {
		t.Fatalf("ExecuteAction advance 2: err = %v, want a *FailureError", err)
	}
	if failure.FinalTime != 2 {
		t.Errorf("ExecuteAction advance 2: FinalTime = %v, want 2", failure.FinalTime)
	}

	run, err := client.ExecuteAction(ctx, model, "Timed::waiter", nil, opensysml.WithAdvance(10))
	if err != nil {
		t.Fatalf("ExecuteAction advance 10: %v", err)
	}
	if run.FinalTime != 10 || sole(t, "fired", run.Outputs["fired"]) != opensysml.Bool(true) {
		t.Errorf("ExecuteAction advance 10: FinalTime = %v, fired = %#v; want 10 and Bool(true)", run.FinalTime, run.Outputs["fired"])
	}

	run, err = client.ExecuteAction(ctx, model, "Timed::waiter", nil, opensysml.WithClockStep(4))
	if err != nil {
		t.Fatalf("ExecuteAction clock step 4: %v", err)
	}
	if run.FinalTime != 8 {
		t.Errorf("ExecuteAction clock step 4: FinalTime = %v, want 8", run.FinalTime)
	}

	state, err := client.ExecuteState(ctx, model, "Timed::Timer", nil, opensysml.WithAdvance(1))
	if err != nil {
		t.Fatalf("ExecuteState advance 1: %v", err)
	}
	if state.FinalTime != 1 || len(state.Visited) != 1 || state.Visited[0] != "armed" || state.Context["fired"] != opensysml.Int(0) {
		t.Errorf("ExecuteState advance 1 = %+v, want the clock at 1, still in armed with fired 0", state)
	}

	_, err = client.ExecuteAction(ctx, model, "Timed::waiter", nil, opensysml.WithDraws("sometimes"))
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Fatalf("ExecuteAction draws sometimes: err = %v, want an invalid-argument *StatusError", err)
	}
}

// DisableLint and EnableLint select the lints Diagnostics reports, per call.
func TestDiagnosticsSelectLints(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, lintedSource)

	codes := func(opts ...opensysml.DiagnosticsOption) map[string]bool {
		t.Helper()
		diagnostics, err := client.Diagnostics(ctx, model, opts...)
		if err != nil {
			t.Fatalf("Diagnostics: %v", err)
		}
		out := map[string]bool{}
		for _, d := range diagnostics {
			out[d.Code] = true
		}
		return out
	}
	if got := codes(); !got["undeclared-signal"] || got["rounded-real-literal"] {
		t.Errorf("default selection = %v, want undeclared-signal without rounded-real-literal", got)
	}
	if got := codes(opensysml.EnableLint("rounded-real-literal")); !got["rounded-real-literal"] {
		t.Errorf("EnableLint selection = %v, want rounded-real-literal", got)
	}
	if got := codes(opensysml.DisableLint("undeclared-signal")); got["undeclared-signal"] {
		t.Errorf("DisableLint selection = %v, want no undeclared-signal", got)
	}
	_, err := client.Diagnostics(ctx, model, opensysml.DisableLint("no-such-lint"))
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Fatalf("Diagnostics unknown lint: err = %v, want an invalid-argument *StatusError", err)
	}
}

// Probe lists the engines after starting the external ones; the built-in
// engines are listed as before.
func TestListEnginesProbe(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	engines, err := client.ListEngines(ctx, opensysml.Probe())
	if err != nil {
		t.Fatalf("ListEngines(Probe()): %v", err)
	}
	names := map[string]bool{}
	for _, engine := range engines {
		names[engine.Name] = true
	}
	for _, want := range []string{"explore", "run", "solve", "sweep"} {
		if !names[want] {
			t.Errorf("probed engines = %v, want %s", names, want)
		}
	}
}
