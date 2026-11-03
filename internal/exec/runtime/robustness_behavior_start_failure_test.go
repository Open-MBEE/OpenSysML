package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessBehaviorStartFailure covers a behavior a type binds to its
// objects that cannot start, or fails while the creation runs it to quiescence: the
// object is created with the failure recorded on that behavior, a verdict reading a
// feature the failed body may write is withheld, and a bound of the runtime's own
// still fails the creation.
func TestRuntimeRobustnessBehaviorStartFailure(t *testing.T) {
	t.Run("machine_without_an_initial_state_is_recorded_and_the_object_stands", testStartFailureMachineRecorded)
	t.Run("action_failing_under_way_is_recorded_with_what_it_wrote", testStartFailureActionRecorded)
	t.Run("failed_machine_performance_life_is_ended", testStartFailureEndsThePerformanceLife)
	t.Run("a_running_machine_is_the_objects_machine_before_a_failed_one", testStartFailureRunningMachinePreferred)
	t.Run("verdict_independent_of_the_failed_body_is_decided", testStartFailureIndependentVerdict)
	t.Run("verdict_reading_what_the_failed_body_may_write_is_undecided", testStartFailureDependentVerdictUndecided)
	t.Run("verdict_reading_what_a_dynamic_write_may_reach_is_undecided", testStartFailureDynamicWriteUndecided)
	t.Run("runtime_bound_reached_still_fails_the_creation", testStartFailureBudgetStillFailsCreation)
	t.Run("failures_are_placed_by_their_owners_among_many_objects", testStartFailuresPlacedByOwner)
}

const relayModel = `package test {
	private import ScalarValues::*;
	part def Relay {
		attribute rating : Real = 10.0;
		exhibit state contacts { state open; state closed; }
	}
	requirement def RatingReq { subject r : Relay; require constraint { r.rating >= 6.0 } }
	requirement ratingOk : RatingReq;
	part relay : Relay;
	satisfy ratingOk by relay;
}`

func testStartFailureMachineRecorded(t *testing.T) {
	ctx, relay, err := instantiateWithLibraries(t, relayModel, "test::relay")
	if !errors.Is(err, errNoInitialState) {
		t.Fatalf("instantiate relay: %v, want the machine with no initial state recorded", err)
	}
	if _, live := ctx.Instance(relay.ID); !live {
		t.Fatal("the object whose machine could not start is not held")
	}
	failed := ctx.StartFailures(relay)
	if len(failed) != 1 || failed[0].Name != "contacts" || failed[0].State == nil {
		t.Fatalf("failed behaviors = %v, want the contacts machine alone, its executor kept for the record", failed)
	}
	if !failed[0].completed() {
		t.Error("the failed machine is not ended")
	}
	if len(ctx.pendingBehaviors) != 0 {
		t.Errorf("%d behavior(s) left queued after the failed start", len(ctx.pendingBehaviors))
	}
	notes := ctx.StartFailureNotes(relay)
	if len(notes) != 1 || !strings.Contains(notes[0].Describe(), "contacts") || !strings.Contains(notes[0].Describe(), "no initial state") {
		t.Errorf("notes = %v, want one naming contacts and its failure", notes)
	}
	if b, ok := relay.ExhibitedState(); !ok || b.Err == nil {
		t.Errorf("ExhibitedState = %v, %t; want the failed machine with its failure recorded", b, ok)
	}
}

func testStartFailureActionRecorded(t *testing.T) {
	ctx, host, err := instantiateWithLibraries(t, `package test {
		private import ScalarValues::*;
		part def Host {
			attribute n : Integer = 0;
			perform action go {
				first start;
				then action write { assign n := 1; }
				then action fail { assign n := n / 0; }
				then done;
			}
		}
		part host : Host;
	}`, "test::host")
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("instantiate host: %v, want the action's division by zero recorded", err)
	}
	if got := featureInt(t, ctx, host, "n"); got != 1 {
		t.Errorf("n = %d, want 1: what the action wrote before failing stands", got)
	}
	failed := ctx.StartFailures(host)
	if len(failed) != 1 || failed[0].Action == nil || !failed[0].completed() {
		t.Fatalf("failed behaviors = %v, want the ended action alone", failed)
	}
	if life, _ := ctx.OccurrenceLife(failed[0].Action.occurrence.ID); life.alive() {
		t.Error("the failed action's performance is still alive")
	}
}

func testStartFailureEndsThePerformanceLife(t *testing.T) {
	ctx, host, err := instantiateWithLibraries(t, `package test {
		private import ScalarValues::*;
		part def Host {
			attribute n : Integer = 0;
			exhibit state life {
				entry; then on;
				state on { entry action crash { assign n := n / 0; } }
			}
		}
		part host : Host;
	}`, "test::host")
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("instantiate host: %v, want the entry action's division by zero recorded", err)
	}
	failed := ctx.StartFailures(host)
	if len(failed) != 1 || failed[0].State == nil {
		t.Fatalf("failed behaviors = %v, want the machine alone", failed)
	}
	if life, _ := ctx.OccurrenceLife(failed[0].State.occurrence.ID); life.alive() {
		t.Error("the failed machine's performance is still alive")
	}
	if waits := ctx.Clock().Waits(); len(waits) != 0 {
		t.Errorf("clock waits = %v after the failure, want none", waits)
	}
}

func testStartFailureRunningMachinePreferred(t *testing.T) {
	ctx, host, err := instantiateWithLibraries(t, `package test {
		part def Host {
			exhibit state broken { state lonely; }
			exhibit state working { entry; then idle; state idle; }
		}
		part host : Host;
	}`, "test::host")
	if !errors.Is(err, errNoInitialState) {
		t.Fatalf("instantiate host: %v, want broken recorded", err)
	}
	machine, ok := host.ExhibitedState()
	if !ok || machine.Name != "working" || machine.Err != nil {
		t.Fatalf("ExhibitedState = %v, %t; want working, the machine that started", machine, ok)
	}
	if failed := ctx.StartFailures(host); len(failed) != 1 || failed[0].Name != "broken" {
		t.Errorf("failed behaviors = %v, want broken alone", failed)
	}
}

// satisfyRelay evaluates the one satisfaction assertion the model states.
func satisfyRelay(t *testing.T, ctx *Context, model string) (bool, error) {
	t.Helper()
	idx := ctx.model.resolver.Index()
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 1 {
		t.Fatalf("%d satisfy assertions, want 1", len(assertions))
	}
	return ctx.EvaluateSatisfaction(assertions[0])
}

func testStartFailureIndependentVerdict(t *testing.T) {
	ctx, _, err := instantiateWithLibraries(t, relayModel, "test::relay")
	if !errors.Is(err, errNoInitialState) {
		t.Fatalf("instantiate relay: %v", err)
	}
	holds, err := satisfyRelay(t, ctx, relayModel)
	if err != nil || !holds {
		t.Fatalf("satisfy = %t, %v; want it to hold: rating is nothing the contacts machine writes", holds, err)
	}
}

func testStartFailureDependentVerdictUndecided(t *testing.T) {
	model := strings.Replace(relayModel, "state closed;", "state closed { entry action derate { assign rating := 0.0; } }", 1)
	ctx, _, err := instantiateWithLibraries(t, model, "test::relay")
	if !errors.Is(err, errNoInitialState) {
		t.Fatalf("instantiate relay: %v", err)
	}
	_, err = satisfyRelay(t, ctx, model)
	var undecided *UndecidedError
	if !errors.Is(err, ErrVerdictUndecided) || !errors.As(err, &undecided) {
		t.Fatalf("satisfy = %v; want it undecided: the contacts machine may still write rating", err)
	}
	if undecided.Feature != "rating" || !strings.Contains(undecided.Behavior, "contacts") {
		t.Errorf("undecided over %q by %q, want rating by the contacts machine", undecided.Feature, undecided.Behavior)
	}
}

func testStartFailureDynamicWriteUndecided(t *testing.T) {
	model := `package test {
		private import ScalarValues::*;
		part def Gauge { attribute reading : Real = 10.0; }
		part def Relay {
			attribute rating : Real = 10.0;
			part gauges : Gauge[2];
			exhibit state contacts {
				state open;
				state closed { entry action derate { assign gauges#(1).reading := 0.0; } }
			}
		}
		requirement def ReadingReq { subject r : Relay; require constraint { r.gauges#(2).reading >= 6.0 } }
		requirement readingOk : ReadingReq;
		part relay : Relay;
		satisfy readingOk by relay;
	}`
	ctx, _, err := instantiateWithLibraries(t, model, "test::relay")
	if !errors.Is(err, errNoInitialState) {
		t.Fatalf("instantiate relay: %v", err)
	}
	if _, err := satisfyRelay(t, ctx, model); !errors.Is(err, ErrVerdictUndecided) {
		t.Fatalf("satisfy = %v; want it undecided: a write through a computed base may reach any gauge", err)
	}
}

func testStartFailureBudgetStillFailsCreation(t *testing.T) {
	ctx, host, err := instantiateWithLibraries(t, `package test {
		private import ScalarValues::*;
		part def Host {
			attribute n : Integer = 0;
			perform action spin { first start; then action s { while true { assign n := n + 1; } } then done; }
		}
		part host : Host;
	}`, "test::host")
	if !exhaustsBudget(err) {
		t.Fatalf("instantiate host: %v, want the runtime's bound reached to fail the creation", err)
	}
	if host != nil || len(ctx.instances) != 0 {
		t.Errorf("host = %v, %d object(s) held; want the creation rolled back", host, len(ctx.instances))
	}
}

// The failures reported beside a new object are placed by their objects' owners: the
// many made before it, failed or not, are not attributed to it, nor its own lost among them.
func testStartFailuresPlacedByOwner(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, crowdModel))
	relay, plain := oneSymbol(t, idx, "test::relay"), oneSymbol(t, idx, "test::plain")
	const crowd = 200
	var last *Instance
	for i := 0; i < crowd; i++ {
		inst, err := ctx.Instantiate(relay)
		if err != nil {
			t.Fatalf("instantiate relay %d: %v", i, err)
		}
		last = inst
	}
	if failed := ctx.StartFailures(last); len(failed) != 1 || failed[0].Object != last {
		t.Fatalf("failed behaviors of the last relay = %v, want its own contacts machine alone", failed)
	}
	inst, err := ctx.Instantiate(plain)
	if err != nil {
		t.Fatalf("instantiate plain: %v", err)
	}
	if failed := ctx.StartFailures(inst); len(failed) != 0 {
		t.Fatalf("failed behaviors of plain = %v, want none: the relays' failures are theirs", failed)
	}
}

// crowdModel is a part whose machine cannot start beside one with no behavior,
// for a context crowded with objects of either.
const crowdModel = `package test {
	part def Relay { exhibit state contacts { state open; state closed; } }
	part def Plain { attribute n = 1; }
	part relay : Relay;
	part plain : Plain;
}`
