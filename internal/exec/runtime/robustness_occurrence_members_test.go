package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessOccurrenceMembers reads the temporal members where a run has
// not produced them: before a start, parked on an accept, after the step budget, across a restore.
func TestRuntimeRobustnessOccurrenceMembers(t *testing.T) {
	t.Run("shots_before_start_are_undetermined_with_reason", testOccurrenceShotsBeforeStart)
	t.Run("shots_of_a_parked_performance", testOccurrenceShotsParked)
	t.Run("reads_after_step_budget", testOccurrenceReadsAfterStepBudget)
	t.Run("restore_takes_back_shots_and_durations", testOccurrenceRestoreTakesBackShots)
	t.Run("image_of_a_receiver_keeps_the_sender_of_a_message_in_flight", testOccurrenceImageKeepsSender)
	t.Run("abandoned_creation_leaves_no_shot_record", testOccurrenceAbandonedCreationShots)
}

const occurrenceWaitSrc = `package Probe {
	private import ScalarValues::*;
	attribute def Never;
	part def P {
		attribute n : Integer = 0;
		perform action act {
			first start;
			then action w accept after 60.0 [SI::s];
			then action m assign n := n + 1;
			then done;
		}
	}
	part def Parked {
		perform action act {
			first start;
			then action w accept Never;
			then done;
		}
	}
	part def Held {
		perform action a { first start; then action w accept Never; then done; }
		perform action b;
		first a then b;
	}
}`

func undeterminedReason(t *testing.T, val Value, err error, what string) string {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if val.Kind != ValUndetermined || val.Undetermined() == nil {
		t.Fatalf("%s = %s, want undetermined", what, FormatValue(val))
	}
	return val.Undetermined().reason
}

func testOccurrenceShotsBeforeStart(t *testing.T) {
	ctx, idx := libraryShapeContext(t, occurrenceWaitSrc)
	h := mustInstantiate(t, ctx, oneSymbol(t, idx, "Probe::Held"))
	// b is held back by the succession from a, which never ends: it is a
	// performance that has not started, so its shots have not happened.
	if val, err := evalOn(t, ctx, h, "b"); err != nil || val.Kind != ValInstance {
		t.Fatalf("b before b starts = %s, %v, want the held performance", FormatValue(val), err)
	}
	for _, expr := range []string{"b.start", "b.snapshots"} {
		val, err := evalOn(t, ctx, h, expr)
		if reason := undeterminedReason(t, val, err, expr); !strings.Contains(reason, "has not started") {
			t.Errorf("%s: reason %q does not say b has not started", expr, reason)
		}
	}
	for _, expr := range []string{"a.done", "Time::DurationOf(a)"} {
		val, err := evalOn(t, ctx, h, expr)
		if reason := undeterminedReason(t, val, err, expr); !strings.Contains(reason, "has not ended") {
			t.Errorf("%s: reason %q does not say the performance has not ended", expr, reason)
		}
	}
	val, err := evalOn(t, ctx, h, "a.successors")
	if reason := undeterminedReason(t, val, err, "a.successors"); !strings.Contains(reason, "has not ended") {
		t.Errorf("a.successors: reason %q does not say a has not ended", reason)
	}
}

// wantOpenSnapshots checks a read of the snapshots of an occurrence that began
// and has not ended: open, with the start shot its one known member.
func wantOpenSnapshots(t *testing.T, val Value, err error, what string) {
	t.Helper()
	reason := undeterminedReason(t, val, err, what)
	if !strings.Contains(reason, "has not ended") {
		t.Errorf("%s: reason %q does not say the occurrence has not ended", what, reason)
	}
	if u := val.Undetermined(); u != nil && (len(u.knownValues()) != 1 || fewestOf(u.Count()) != 2) {
		t.Errorf("%s = %s, want the start snapshot known and at least one more", what, FormatValue(val))
	}
}

func testOccurrenceShotsParked(t *testing.T) {
	ctx, idx := libraryShapeContext(t, occurrenceWaitSrc)
	p := mustInstantiate(t, ctx, oneSymbol(t, idx, "Probe::Parked"))
	if val := mustEvalOn(t, ctx, p, "act.start"); val.Kind != ValInstance {
		t.Errorf("act.start = %s, want the start snapshot", FormatValue(val))
	}
	val, err := evalOn(t, ctx, p, "act.done")
	if reason := undeterminedReason(t, val, err, "act.done"); !strings.Contains(reason, "has not ended") {
		t.Errorf("act.done: reason %q does not say the performance has not ended", reason)
	}
	val, err = evalOn(t, ctx, p, "Time::DurationOf(act)")
	if reason := undeterminedReason(t, val, err, "Time::DurationOf(act)"); !strings.Contains(reason, "has not ended") {
		t.Errorf("DurationOf(act): reason %q does not say the performance has not ended", reason)
	}
	val, err = evalOn(t, ctx, p, "act.snapshots")
	wantOpenSnapshots(t, val, err, "act.snapshots")
	// Nothing ended before act began but its holder's start shot; no succession fired.
	if got := FormatValue(mustEvalOn(t, ctx, p, "act.predecessors->SequenceFunctions::size()")); got != "1" {
		t.Errorf("act.predecessors has %s elements, want the holder's start shot alone", got)
	}
	if got := FormatValue(mustEvalOn(t, ctx, p, "act.predecessors->SequenceFunctions::includes(self.start)")); got != "true" {
		t.Errorf("act.predecessors includes the holder's start shot = %s, want true", got)
	}
}

func testOccurrenceReadsAfterStepBudget(t *testing.T) {
	ctx, idx := libraryShapeContext(t, occurrenceWaitSrc)
	p := mustInstantiate(t, ctx, oneSymbol(t, idx, "Probe::P"))
	// The budget runs out where the run stands: moving the clock to the wait's
	// end is refused with the typed error, and the shots stay where the run was.
	ctx.maxSteps = 1
	if _, err := ctx.Advance(60); !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("Advance at an exhausted budget = %v, want ErrStepLimitExceeded", err)
	}
	ctx.maxSteps = 100000
	if val := mustEvalOn(t, ctx, p, "act.start"); val.Kind != ValInstance {
		t.Errorf("act.start after the budget = %s, want the start snapshot", FormatValue(val))
	}
	val, err := evalOn(t, ctx, p, "act.done")
	if reason := undeterminedReason(t, val, err, "act.done after the budget"); !strings.Contains(reason, "has not ended") {
		t.Errorf("act.done after the budget: reason %q", reason)
	}
	val, err = evalOn(t, ctx, p, "act.snapshots")
	wantOpenSnapshots(t, val, err, "act.snapshots after the budget")
}

func testOccurrenceRestoreTakesBackShots(t *testing.T) {
	ctx, idx := libraryShapeContext(t, occurrenceWaitSrc)
	p := mustInstantiate(t, ctx, oneSymbol(t, idx, "Probe::P"))
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	defer snapshot.Release()
	if _, err := ctx.Advance(60); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	done := mustEvalOn(t, ctx, p, "act.done")
	if done.Kind != ValInstance {
		t.Fatalf("act.done after the wait = %s, want the end snapshot", FormatValue(done))
	}
	if got := FormatValue(mustEvalOn(t, ctx, p, "Time::DurationOf(act)")); !strings.HasPrefix(got, "60.0") {
		t.Errorf("DurationOf(act) after the wait = %s, want 60.0 [s]", got)
	}
	snapshot.Restore()
	if _, live := ctx.instances[done.Instance]; live {
		t.Errorf("end snapshot #%d made after the snapshot is still held", done.Instance)
	}
	val, err := evalOn(t, ctx, p, "act.done")
	if reason := undeterminedReason(t, val, err, "act.done after the restore"); !strings.Contains(reason, "has not ended") {
		t.Errorf("act.done after the restore: reason %q", reason)
	}
	val, err = evalOn(t, ctx, p, "Time::DurationOf(act)")
	undeterminedReason(t, val, err, "DurationOf(act) after the restore")
	val, err = evalOn(t, ctx, p, "act.snapshots")
	wantOpenSnapshots(t, val, err, "act.snapshots after the restore")
	if _, err := ctx.Advance(60); err != nil {
		t.Fatalf("Advance after the restore: %v", err)
	}
	if again := mustEvalOn(t, ctx, p, "act.done"); again.Kind != ValInstance {
		t.Errorf("act.done after running again = %s, want the end snapshot", FormatValue(again))
	}
}

const occurrenceInFlightSrc = `package Probe {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	attribute def Ping;
	attribute def Pong;
	part def Receiver {
		perform action rcv {
			first start;
			then action r accept Pong;
			then done;
		}
	}
	part def Sender {
		part r : Receiver;
		perform action snd {
			first start;
			then action s send new Ping() to r;
			then done;
		}
	}
}`

func testOccurrenceImageKeepsSender(t *testing.T) {
	ctx, idx := libraryShapeContext(t, occurrenceInFlightSrc)
	s := mustInstantiate(t, ctx, oneSymbol(t, idx, "Probe::Sender"))
	// The Ping stays in flight: the receiver accepts only a Pong.
	if sent, err := evalOn(t, ctx, s, "size(r.incomingTransfers)"); err != nil || FormatValue(sent) != "1" {
		t.Fatalf("r.incomingTransfers = %s, %v, want the one send in flight", FormatValue(sent), err)
	}
	r, err := evalOn(t, ctx, s, "r")
	if err != nil {
		t.Fatal(err)
	}
	dst := imageInto(t, ctx, ctx.instances[r.Instance])
	var receiver *Instance
	for _, id := range dst.InstanceIDs() {
		if inst := dst.instances[id]; SymbolText(inst.Type) == "Receiver" {
			receiver = inst
		}
	}
	if receiver == nil {
		types := []string{}
		for _, id := range dst.InstanceIDs() {
			types = append(types, SymbolText(dst.instances[id].Type))
		}
		t.Fatalf("the image holds no receiver: %v", types)
	}
	source, err := evalOn(t, dst, receiver, "incomingTransfers#(1).source")
	if err != nil || source.Kind != ValInstance {
		t.Fatalf("restored incomingTransfers#(1).source = %s, %v, want the sending performance", FormatValue(source), err)
	}
	if typ := SymbolText(dst.instances[source.Instance].Type); typ != "snd" {
		t.Errorf("restored transfer source is a %s, want the sending performance snd", typ)
	}
}

func testOccurrenceAbandonedCreationShots(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package Probe {
		private import ScalarValues::*;
		part def Base { attribute v : Integer = 1; }
		part def Holder {
			part base : Base;
			attribute v : Integer = base.v;
		}
	}`)
	h := mustInstantiate(t, ctx, oneSymbol(t, idx, "Probe::Holder"))
	base, err := evalOn(t, ctx, h, "base")
	if err != nil {
		t.Fatal(err)
	}
	// A creation that fails after reading the shot abandons everything made since its mark,
	// the shot object included; the record of it must go with it.
	mark := len(ctx.created)
	first, err := evalOn(t, ctx, h, "base.start")
	if err != nil || first.Kind != ValInstance {
		t.Fatalf("base.start = %s, %v, want a shot of base", FormatValue(first), err)
	}
	ctx.abandonInstancesSince(mark)
	if shots := ctx.shots[base.Instance]; shots.start != 0 {
		t.Fatalf("base records shot #%d after the object was abandoned", shots.start)
	}
	again, err := evalOn(t, ctx, h, "base.start")
	if err != nil || again.Kind != ValInstance || ctx.instances[again.Instance] == nil {
		t.Fatalf("base.start after the abandonment = %s, %v, want a live shot of base", FormatValue(again), err)
	}
	if again.Instance == first.Instance {
		t.Errorf("base.start is the abandoned object #%d again", first.Instance)
	}
}
