package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessStateEventQueries exercises what the state and event
// document queries read at a machine that is over: a terminated one holds no
// active configuration, a completed one keeps its final state, Destroyed
// reports the instant a `destroy` ended an object, and a bounded recorder
// reports what it dropped.
func TestRuntimeRobustnessStateEventQueries(t *testing.T) {
	t.Run("terminated_machine_reports_no_active_state", testTerminatedMachineNoActiveState)
	t.Run("completed_machine_reports_its_final_state", testCompletedMachineFinalState)
	t.Run("destroyed_reports_the_instant", testDestroyedReportsTheInstant)
	t.Run("bounded_recorder_drops_the_oldest_records", testBoundedRecorderDropsOldest)
}

const endedMachineModel = `package test {
	private import ScalarValues::*;
	attribute def Abort;
	state def FinishingMachine {
		entry; then working;
		state working;
		transition first working accept Abort then done;
	}
	state def TerminatingMachine {
		entry; then busy;
		state busy;
		transition first busy accept Abort then stop;
		action stop terminate;
	}
}`

// A machine a `terminate` ended has no active states or leaves and no current
// state, so a reader enumerating them sees nothing rather than a stale state.
func testTerminatedMachineNoActiveState(t *testing.T) {
	ctx, sym := loadState(t, endedMachineModel, "TerminatingMachine")
	exec, err := ctx.PerformState(sym, nil, []QueuedEvent{{Signal: "Abort"}})
	if err != nil {
		t.Fatalf("PerformState: %v", err)
	}
	if got := exec.ActiveStates(); len(got) != 0 {
		t.Fatalf("ActiveStates of a terminated machine = %d, want none", len(got))
	}
	if got := exec.ActiveLeaves(); len(got) != 0 {
		t.Fatalf("ActiveLeaves of a terminated machine = %d, want none", len(got))
	}
	if got := exec.CurrentState(); got != nil {
		t.Fatalf("CurrentState of a terminated machine = %v, want nil", got)
	}
}

// A machine that reached `done` keeps it as the active configuration.
func testCompletedMachineFinalState(t *testing.T) {
	ctx, sym := loadState(t, endedMachineModel, "FinishingMachine")
	exec, err := ctx.PerformState(sym, nil, []QueuedEvent{{Signal: "Abort"}})
	if err != nil {
		t.Fatalf("PerformState: %v", err)
	}
	leaves := exec.ActiveLeaves()
	if len(leaves) != 1 || leaves[0].Name != "done" {
		t.Fatalf("ActiveLeaves of a completed machine = %v, want [done]", leaves)
	}
	if exec.CurrentState() == nil {
		t.Fatalf("CurrentState of a completed machine = nil, want done")
	}
}

// Destroyed reports nothing for a living object and the destruction instant
// after `destroy` ran.
func testDestroyedReportsTheInstant(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		part def Widget { attribute n : Integer = 1; }
		part w : Widget;
	}`))
	sym := namedSymbol(idx, "test::w", ast.DefPart, ast.UsagePart)
	if sym == nil {
		t.Fatal("part w not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate w: %v", err)
	}
	if at, destroyed := ctx.Destroyed(inst); destroyed || at != 0 {
		t.Fatalf("Destroyed before destroy = (%d, %v), want (0, false)", at, destroyed)
	}
	if err := ctx.destroy(inst); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	at, destroyed := ctx.Destroyed(inst)
	if !destroyed || at == 0 {
		t.Fatalf("Destroyed after destroy = (%d, %v), want (instant, true)", at, destroyed)
	}
	if life, ok := ctx.OccurrenceLife(inst.ID); !ok || !life.Destroyed || life.Ended != at {
		t.Fatalf("OccurrenceLife = %+v, %v; want destroyed at %d", life, ok, at)
	}
}

// A bounded recorder keeps the most recent records and reports how many it
// dropped and the instant history begins at.
func testBoundedRecorderDropsOldest(t *testing.T) {
	recorder := NewEventRecorder(2)
	for i, kind := range []TraceKind{TraceAccept, TraceEntry, TraceExit} {
		recorder.add(TraceRecord{Kind: kind, Origin: TraceOrigin{At: float64(i)}})
	}
	records := recorder.Records()
	if len(records) != 2 || records[0].Kind != TraceEntry || records[1].Kind != TraceExit {
		t.Fatalf("records = %v, want the 2 most recent", records)
	}
	if dropped, upTo := recorder.Dropped(); dropped != 1 || upTo != 0 {
		t.Fatalf("Dropped = (%d, %v), want (1, 0)", dropped, upTo)
	}
}
