package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"testing"
	"weak"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// snapshotPausedBodyCases are the conformance cases whose default run pauses a
// body mid-statement at some step: a do behavior waiting on a message or the
// clock. A snapshot there fails with ErrSnapshotPausedBody, which the round-trip
// test pins; the steps before and after it round-trip as every other case's do.
var snapshotPausedBodyCases = map[string]bool{
	"action_explore_performed_and_accept_due_together": true,
	"state_concurrent_do_action_bodies_timed":          true,
	"state_do_action_declaration_order":                true,
	"state_do_action_signal_accept_cancelled_on_exit":  true,
	"state_do_action_timed_accept_cancelled_on_exit":   true,
	"state_do_action_typed_inout_cancelled_on_exit":    true,
	"state_do_action_typed_inout_valued_by_a_literal":  true,
	"state_do_action_typed_inout_writes_back":          true,
}

// steppedRun drives one conformance case the way the trace harness does, one
// step at a time, so a snapshot can be taken between any two steps.
type steppedRun struct {
	ctx   *Context
	trace *TraceRecorder
	// snapshot captures the run: through the executor it steps, or the context.
	snapshot func() (*Snapshot, error)
	// step advances the run by one step; false when the run has no step left
	// before finish takes it to completion.
	step func() (bool, error)
	// taken counts the steps taken since the run began, set back by restoreTo.
	taken int
	// stepErr is the error the step numbered errAt returned, part of the outcome.
	stepErr error
	errAt   int
	// finish runs the case to completion, as the trace harness does.
	finish func() error
	// executors are the ones the run stepped, whose state the digest reports.
	actions []*ActionExecutor
	states  []*StateExecutor
}

// roundTripOutcome is what a run left behind: its trace, its objects' values and its error.
type roundTripOutcome struct {
	trace, values, err string
}

// advance takes one step, counting it.
func (r *steppedRun) advance() (bool, error) {
	r.taken++
	more, err := r.step()
	if err != nil && r.stepErr == nil {
		r.stepErr, r.errAt = err, r.taken
	}
	return more, err
}

// restoreTo restores the run to a snapshot taken after `taken` steps.
func (r *steppedRun) restoreTo(snapshot *Snapshot, taken int) {
	snapshot.Restore()
	r.taken = taken
	if taken < r.errAt {
		r.stepErr, r.errAt = nil, 0
	}
}

// resume takes the steps up to the one numbered taken, as the plain run did.
func (r *steppedRun) resume(taken int) {
	for r.taken < taken {
		_, _ = r.advance()
	}
}

// complete runs the case to completion; the outcome's error is the first the
// steps or the completion returned.
func (r *steppedRun) complete() roundTripOutcome {
	err := r.finish()
	if r.stepErr != nil {
		err = r.stepErr
	}
	out := roundTripOutcome{trace: r.trace.String(), values: r.digest()}
	if err != nil {
		out.err = err.Error()
	}
	return out
}

// digest renders every object the run made, its lifetime, the bus, the clock
// and the stepped executors' state, so two runs can be compared beyond their traces.
func (r *steppedRun) digest() string {
	var b strings.Builder
	ctx := r.ctx
	for _, id := range ctx.created {
		inst := ctx.instances[id]
		if inst == nil {
			continue
		}
		fmt.Fprintf(&b, "#%d %s life=%+v\n", id, symbolText(inst.Type), ctx.lives[id])
		names := make([]string, 0, len(inst.FeatureValues))
		for name := range inst.FeatureValues {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fv := inst.FeatureValues[name]
			fmt.Fprintf(&b, "  %s = %s written=%t materialized=%t\n", name, FormatValue(fv.HeldValue()), fv.Written, fv.Materialized)
		}
		for _, behavior := range inst.behaviors {
			fmt.Fprintf(&b, "  behavior %s\n", behavior.Name)
		}
	}
	fmt.Fprintf(&b, "clock=%v waiters=%d\n", ctx.clock.now, len(ctx.clock.waiters))
	for _, msg := range ctx.messages {
		fmt.Fprintf(&b, "message %s -> %s\n", msg.SignalType, msg.Target)
	}
	fmt.Fprintf(&b, "activations=%d runs=%d steps=%d\n", ctx.activations, ctx.runs, ctx.run.steps)
	for _, exec := range r.actions {
		fmt.Fprintf(&b, "action %s state=%v tokens=%d results=%s\n", symbolText(exec.action), exec.State(), len(exec.tokens), formatValues(exec.Results()))
		for _, token := range exec.tokens {
			fmt.Fprintf(&b, "  token %d at %s\n", token.ID, ActionNodeName(token.Location))
		}
	}
	for _, exec := range r.states {
		var active []string
		for _, state := range exec.ActiveStates() {
			active = append(active, getNodeName(state))
		}
		fmt.Fprintf(&b, "state %s state=%v active=%v stack=%d queue=%d deferred=%d data=%s\n",
			symbolText(exec.stateMachine), exec.State(), active, len(exec.stateStack), exec.eventQueue.Len(), len(exec.deferred), formatValues(exec.StateData()))
	}
	return b.String()
}

func formatValues(values map[string]Value) string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = name + "=" + FormatValue(values[name])
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// noSteps is the step of a run whose case is taken in one call.
func noSteps() (bool, error) { return false, nil }

// newSteppedRun loads a conformance case and sets up its run under the default
// policy, following runTraceTest's driving of each case kind.
func newSteppedRun(t *testing.T, conformanceDir, testName string, expected ExpectedOutcome) *steppedRun {
	t.Helper()
	ctx, idx, rootScope := loadTraceCase(t, conformanceDir, testName, expected, DefaultSchedulePolicy)
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)
	run := &steppedRun{ctx: ctx, trace: trace, snapshot: ctx.Snapshot, step: noSteps}

	var actionEntry, stateEntry string
	switch expected.Type {
	case "calc":
		calcSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefCalc, ast.UsageCalc)
		args := make([]Value, len(expected.Inputs))
		for i, input := range expected.Inputs {
			args[i] = expectedToRuntimeValue(t, input)
		}
		run.finish = func() error { _, err := ctx.InvokeCalc(calcSym, args, rootScope); return err }
	case "calcUsage":
		usageSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefCalc, ast.UsageCalc)
		run.finish = func() error { _, err := ctx.CalcUsageOutputs(usageSym, usageSym.OwnerScope, nil); return err }
	case "analysis":
		caseSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefAnalysisCase, ast.UsageAnalysisCase)
		run.finish = func() error {
			_, err := ctx.RunAnalysis(caseSym, analysisArgsOf(t, ctx, idx, expected), rootScope, nil)
			return err
		}
	case "verification":
		caseSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefVerificationCase, ast.UsageVerificationCase)
		run.finish = func() error {
			_, err := ctx.RunVerification(caseSym, analysisArgsOf(t, ctx, idx, expected), rootScope, nil)
			return err
		}
	case "constraint":
		constraintSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefConstraint, ast.UsageConstraint)
		run.finish = func() error { _, err := ctx.EvaluateConstraint(constraintSym, rootScope); return err }
	case "requirement", "satisfy":
		// The conformance harness checks these cases' verdicts itself; the round
		// trip compares what the checks leave behind.
		sysmlPath := filepath.Join(conformanceDir, testName+".sysml")
		check := runRequirementConformance
		if expected.Type == "satisfy" {
			check = runSatisfyConformance
		}
		run.finish = func() error { check(t, ctx, idx, sysmlPath, expected); return nil }
	case "instance":
		// Materializing the object is the one step; the machines of its objects
		// run to completion after it.
		typeSym := oneSymbol(t, idx, expected.Instantiate)
		var inst *Instance
		run.step = func() (bool, error) {
			var err error
			inst, err = ctx.Instantiate(typeSym)
			return false, err
		}
		run.finish = func() error {
			if run.taken == 0 {
				var err error
				if inst, err = ctx.Instantiate(typeSym); err != nil {
					return err
				}
			}
			return runObjectMachines(t, ctx, inst, expected)
		}
	case "action":
		actionEntry = expected.Evaluate
	case "state":
		stateEntry = expected.Evaluate
	}
	actionSym := entryBehavior(idx, actionEntry, rootScope, ast.DefAction, ast.UsageAction)
	stateSym := entryBehavior(idx, stateEntry, rootScope, ast.DefState, ast.UsageState)

	// The trace harness runs a behavior it finds beside a case of another kind
	// after the case; here that run completes the case, and the steps are the case's.
	if run.finish != nil {
		after := run.finish
		run.finish = func() error {
			if err := after(); err != nil {
				return err
			}
			if actionSym != nil {
				if _, err := ctx.ExecuteAction(actionSym); err != nil {
					return err
				}
			}
			if stateSym != nil && len(expected.Performers) == 0 {
				exec, err := ctx.CreateStateExecutor(stateSym)
				if err != nil {
					return err
				}
				injectEvents(t, exec, expected.Events)
				return exec.RunToCompletion()
			}
			return nil
		}
		return run
	}

	// The behavior of the case's kind is stepped; one of the other kind found
	// beside it runs to completion after it, as the trace harness runs both.
	var after func() error
	switch {
	case stateSym != nil && (expected.Type == "state" || actionSym == nil):
		if actionSym != nil {
			after = func() error { _, err := ctx.ExecuteAction(actionSym); return err }
		}
		run.stepState(t, idx, stateSym, expected)
	case actionSym != nil:
		if stateSym != nil {
			after = func() error { return runStateEntry(t, ctx, stateSym, expected) }
		}
		run.stepAction(actionSym)
	default:
		t.Fatalf("%s drives no behavior", testName)
	}
	if after != nil {
		stepped := run.finish
		run.finish = func() error {
			if err := stepped(); err != nil {
				return err
			}
			return after()
		}
	}
	return run
}

// stepAction drives the action executor built for sym one step at a time;
// parked on the clock or a message, RunToCompletion moves it as the trace harness does.
func (r *steppedRun) stepAction(sym *symbols.Symbol) {
	ctx := r.ctx
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		// A case whose executor fails to build is that failure, taken again.
		r.finish = func() error { _, err := ctx.CreateActionExecutor(sym); return err }
		return
	}
	exec.SetTrace(r.trace)
	r.actions = append(r.actions, exec)
	r.snapshot = exec.Snapshot
	r.step = func() (bool, error) {
		err := exec.Step()
		return err == nil && exec.State() == StateRunning, err
	}
	r.finish = exec.RunToCompletion
}

// stepState drives the machine built for sym one dispatched event at a time. A
// machine with no event left is Running still, so the step reports no event as
// the end of the steps.
func (r *steppedRun) stepState(t *testing.T, idx *symbols.Index, sym *symbols.Symbol, expected ExpectedOutcome) {
	ctx := r.ctx
	if len(expected.Performers) > 0 {
		r.finish = func() error { return runPerformers(t, r, idx, sym, expected.Performers) }
		return
	}
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		r.finish = func() error { _, err := ctx.CreateStateExecutor(sym); return err }
		return
	}
	exec.SetTrace(r.trace)
	injectEvents(t, exec, expected.Events)
	r.states = append(r.states, exec)
	r.snapshot = exec.Snapshot
	r.step = func() (bool, error) {
		err := exec.ProcessNextEvent()
		return err == nil && exec.State() == StateRunning && exec.eventQueue.Len() > 0, err
	}
	r.finish = exec.RunToCompletion
}

// runStateEntry runs the machine built for sym to completion under the case's events.
func runStateEntry(t *testing.T, ctx *Context, sym *symbols.Symbol, expected ExpectedOutcome) error {
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		return err
	}
	injectEvents(t, exec, expected.Events)
	return exec.RunToCompletion()
}

// runObjectMachines runs the machines of the objects an instance case names, as
// traceObjectRuns does, returning the first error rather than failing the test.
func runObjectMachines(t *testing.T, ctx *Context, first *Instance, expected ExpectedOutcome) error {
	t.Helper()
	if len(expected.Objects) == 0 {
		return nil
	}
	objects := materializations(t, ctx, first.Type, first, expected.Objects)
	for _, run := range expected.Objects {
		obj := objects[instanceIndexOf(run)]
		if run.Path != "" {
			obj = instanceAtPath(t, ctx, obj, run.Path)
		}
		exec := objectMachine(t, obj, run.Behavior)
		injectEvents(t, exec, run.Events)
		if err := exec.RunToCompletion(); err != nil {
			return err
		}
	}
	return nil
}

// runPerformers runs a state machine once per performing object, as tracePerformers does.
func runPerformers(t *testing.T, run *steppedRun, idx *symbols.Index, stateSym *symbols.Symbol, performers []Performer) error {
	t.Helper()
	for _, performer := range performers {
		self, err := run.ctx.Instantiate(oneSymbol(t, idx, performer.Object))
		if err != nil {
			return err
		}
		exec, err := run.ctx.CreateStateExecutorFor(stateSym, self)
		if err != nil {
			return err
		}
		injectEvents(t, exec, performer.Events)
		if err := exec.RunToCompletion(); err != nil {
			return err
		}
	}
	return nil
}

// maxSnapshotSteps bounds the steps a case is snapshotted at, so a machine
// stepping through a long loop does not make the round trip quadratic in it.
const maxSnapshotSteps = 400

// forEachConformanceCase runs f for every conformance case the trace harness drives.
func forEachConformanceCase(t *testing.T, f func(t *testing.T, conformanceDir, testName string, expected ExpectedOutcome)) {
	t.Helper()
	conformanceDir := filepath.Join("testdata", "conformance")
	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("read conformance dir: %v", err)
	}
	knownFailures := loadKnownFailures(t, conformanceDir)
	cases := 0
	for _, entry := range entries {
		if entry.IsDir() || !isConformanceCase(entry.Name()) {
			continue
		}
		testName := strings.TrimSuffix(entry.Name(), ".expected.json")
		if knownFailures[testName] {
			continue
		}
		expected := loadExpectedOutcome(t, conformanceDir, testName)
		cases++
		t.Run(testName, func(t *testing.T) { f(t, conformanceDir, testName, expected) })
	}
	if cases == 0 {
		t.Fatalf("no conformance cases in %s", conformanceDir)
	}
}

// TestSnapshotRoundTrip snapshots every conformance case at every step of its
// default run, runs it to completion, then restores each snapshot and runs to
// completion again: the same trace, the same values, the same error every time,
// and the same as a run never snapshotted.
func TestSnapshotRoundTrip(t *testing.T) {
	forEachConformanceCase(t, func(t *testing.T, conformanceDir, testName string, expected ExpectedOutcome) {
		// The plain run fixes how many steps are taken before completion: every
		// step there is, the one that fails included, up to the bound.
		plain := newSteppedRun(t, conformanceDir, testName, expected)
		for more := true; more && plain.taken < maxSnapshotSteps; {
			var err error
			if more, err = plain.advance(); err != nil {
				break
			}
		}
		reference := plain.complete()

		run := newSteppedRun(t, conformanceDir, testName, expected)
		snapshots := make(map[int]*Snapshot)
		paused := false
		for i := 0; ; i++ {
			snapshot, err := run.snapshot()
			switch {
			case err == nil:
				snapshots[i] = snapshot
			case errors.Is(err, ErrSnapshotPausedBody) && snapshotPausedBodyCases[testName]:
				paused = true
			default:
				t.Fatalf("snapshot at step %d: %v", i, err)
			}
			if i == plain.taken {
				break
			}
			run.resume(i + 1)
		}
		if snapshotPausedBodyCases[testName] && !paused {
			t.Fatalf("%s is pinned as pausing a body mid-statement, but every step snapshotted", testName)
		}
		snapshotted := run.complete()
		if snapshotted != reference {
			t.Fatalf("taking snapshots changed the run\n%s", outcomeDiff(reference, snapshotted))
		}
		for i := plain.taken; i >= 0; i-- {
			snapshot := snapshots[i]
			if snapshot == nil {
				continue
			}
			run.restoreTo(snapshot, i)
			run.resume(plain.taken)
			if restored := run.complete(); restored != reference {
				t.Fatalf("restored to snapshot at step %d of %d\n%s", i, plain.taken, outcomeDiff(reference, restored))
			}
		}
		for _, snapshot := range snapshots {
			snapshot.Release()
		}
		if run.ctx.journals != 0 || len(run.ctx.journalWrites)+len(run.ctx.journalUndos) != 0 {
			t.Fatalf("journal left open after every snapshot was released: %d journals, %d writes, %d undos",
				run.ctx.journals, len(run.ctx.journalWrites), len(run.ctx.journalUndos))
		}
	})
}

// TestSnapshotRestoresTwice restores one snapshot of every conformance case
// twice, running to completion from each: a snapshot is not consumed by a restore.
func TestSnapshotRestoresTwice(t *testing.T) {
	forEachConformanceCase(t, func(t *testing.T, conformanceDir, testName string, expected ExpectedOutcome) {
		run := newSteppedRun(t, conformanceDir, testName, expected)
		// Snapshot before the first step, then every other step after it, so the
		// snapshot kept is the latest one the run has state to restore to.
		snapshot, err := run.snapshot()
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		at := 0
		for more := true; more && run.taken < maxSnapshotSteps; {
			if more, err = run.advance(); err != nil {
				break
			}
			if run.taken%2 != 0 {
				continue
			}
			later, err := run.snapshot()
			if err != nil {
				if errors.Is(err, ErrSnapshotPausedBody) && snapshotPausedBodyCases[testName] {
					continue
				}
				t.Fatalf("snapshot at step %d: %v", run.taken, err)
			}
			snapshot.Release()
			snapshot, at = later, run.taken
		}
		taken := run.taken
		first := run.complete()
		run.restoreTo(snapshot, at)
		run.resume(taken)
		second := run.complete()
		run.restoreTo(snapshot, at)
		run.resume(taken)
		third := run.complete()
		snapshot.Release()
		if second != first {
			t.Fatalf("first restore\n%s", outcomeDiff(first, second))
		}
		if third != first {
			t.Fatalf("second restore\n%s", outcomeDiff(first, third))
		}
	})
}

// A usage instantiated again after the snapshot denotes the new object; restoring
// makes it denote the object it denoted at the snapshot, not none and not a third.
func TestSnapshotRestoresAnOverwrittenOccurrence(t *testing.T) {
	ctx, idx := contextForSource(t, `package Demo {
	part def Car { attribute wheels = 4; }
	part car : Car;
}`)
	car := lookupOne(t, idx, "Demo::car")
	first, err := ctx.Instantiate(car)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	second, err := ctx.Instantiate(car)
	if err != nil {
		t.Fatalf("Instantiate again: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("second Instantiate answered the first object #%d", first.ID)
	}
	for round := 1; round <= 2; round++ {
		snapshot.Restore()
		denoted, err := ctx.occurrenceOf(car)
		if err != nil {
			t.Fatalf("restore %d: occurrenceOf: %v", round, err)
		}
		if denoted != first {
			t.Fatalf("restore %d: Demo::car denotes #%d, want the snapshotted #%d", round, denoted.ID, first.ID)
		}
		if _, live := ctx.instances[second.ID]; live {
			t.Fatalf("restore %d: the object made after the snapshot, #%d, is still held", round, second.ID)
		}
	}
	snapshot.Release()
}

const sharedIdentitiesSrc = `package Demo {
	part def Car;
	part car : Car;
}`

// instantiateCar materializes Demo::car in ctx and answers its identity.
func instantiateCar(t *testing.T, ctx *Context, idx *symbols.Index) int64 {
	t.Helper()
	inst, err := ctx.Instantiate(lookupOne(t, idx, "Demo::car"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return inst.ID
}

// Restoring a snapshot never hands out again an identity a context sharing the
// sequence took since: the other context keeps its object, so this one takes the next.
func TestSnapshotRestoreKeepsIdentitiesASharingContextTook(t *testing.T) {
	prev, prevIdx := contextForSource(t, sharedIdentitiesSrc)
	ctx, idx := contextForSource(t, sharedIdentitiesSrc)
	ctx.AdoptIdentities(prev)
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	taken := instantiateCar(t, prev, prevIdx)
	snapshot.Restore()
	if got := instantiateCar(t, ctx, idx); got <= taken {
		t.Fatalf("after restore the context handed out #%d, which the sharing context took #%d at or past", got, taken)
	}
	snapshot.Release()
}

// A context that takes over another's identity sequence after the snapshot keeps
// the shared sequence on restore rather than the one it had, so the two never
// name one identity for two objects.
func TestSnapshotRestoreKeepsAnAdoptedIdentitySequence(t *testing.T) {
	prev, prevIdx := contextForSource(t, sharedIdentitiesSrc)
	ctx, idx := contextForSource(t, sharedIdentitiesSrc)
	own := instantiateCar(t, ctx, idx)
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	made := instantiateCar(t, ctx, idx)
	taken := instantiateCar(t, prev, prevIdx)
	ctx.AdoptIdentities(prev)
	snapshot.Restore()
	if ctx.ids != prev.ids {
		t.Fatalf("restore reinstalled the sequence the context had before it adopted the other's")
	}
	if _, live := ctx.instances[made]; live {
		t.Fatalf("restore kept #%d, made after the snapshot", made)
	}
	got := instantiateCar(t, ctx, idx)
	if got <= taken || got <= made {
		t.Fatalf("after restore the context handed out #%d; it holds #%d, the other context #%d, and #%d was handed out since", got, own, taken, made)
	}
	if _, live := prev.instances[got]; live {
		t.Fatalf("#%d names an object in both contexts", got)
	}
	snapshot.Release()
}

// The sequence keeps no context it was shared with: a replaced context is
// collected once dropped, and what it took is still never handed out again.
func TestAdoptIdentitiesKeepsNoReplacedContext(t *testing.T) {
	ctx, idx := contextForSource(t, sharedIdentitiesSrc)
	replaced, taken, snapshot := adoptThenDrop(t, ctx)
	goruntime.GC()
	goruntime.GC()
	if replaced.Value() != nil {
		t.Fatalf("the replaced context is still held after it was dropped")
	}
	snapshot.Restore()
	if got := instantiateCar(t, ctx, idx); got <= taken {
		t.Fatalf("after restore the context handed out #%d, which the dropped context took #%d at or past", got, taken)
	}
	snapshot.Release()
}

// adoptThenDrop has ctx adopt the identities of a fresh context, snapshots ctx,
// has the fresh context take an identity and drops it, keeping only a weak pointer.
func adoptThenDrop(t *testing.T, ctx *Context) (weak.Pointer[Context], int64, *Snapshot) {
	t.Helper()
	prev, prevIdx := contextForSource(t, sharedIdentitiesSrc)
	ctx.AdoptIdentities(prev)
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	taken := instantiateCar(t, prev, prevIdx)
	return weak.Make(prev), taken, snapshot
}

// Restoring a snapshot forgets the outputs an open calc usage evaluation worked
// out since: the evaluation stays its activation's, holding what it had at the mark.
func TestSnapshotRestoreForgetsCalcOutputsWorkedOutSince(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package Demo {
		private import ScalarValues::*;
		calc def Pair { in x : Integer; out a : Integer = x + 1; out b : Integer = x + 2; }
		calc pair : Pair { in x = 1; }
	}`)
	pair := lookupOne(t, idx, "Demo::pair")
	end := ctx.beginRun()
	reader := NewEvalContextIn(ctx, pair.OwnerScope, nil)
	reader.activation = ctx.newActivation()
	run, err := ctx.calcUsageRun(reader, pair)
	if err != nil {
		t.Fatalf("calcUsageRun: %v", err)
	}
	if _, err := run.output(ctx, "a"); err != nil {
		t.Fatalf("output a: %v", err)
	}
	end()
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if _, err := run.output(ctx, "b"); err != nil {
		t.Fatalf("output b: %v", err)
	}
	snapshot.Restore()
	if held := ctx.run.calcUsageRuns[reader.activation][calcUsageKey{sym: pair}]; held != run {
		t.Fatalf("restore replaced the activation's evaluation of Demo::pair")
	}
	if _, held := run.outputs["b"]; held {
		t.Fatalf("restore kept output b, worked out after the snapshot")
	}
	if _, held := run.outputs["a"]; !held {
		t.Fatalf("restore dropped output a, worked out before the snapshot")
	}
	snapshot.Release()
}

// Restoring a snapshot brings the trace back to the mark: entries cleared since
// return, recording turned off since is on again, and the next entry is recorded
// at the nesting the statement open at the mark holds.
func TestSnapshotRestoreBringsTheTraceBackToTheMark(t *testing.T) {
	ctx, _ := contextForSource(t, sharedIdentitiesSrc)
	tr := NewTraceRecorder()
	ctx.SetTrace(tr)
	tr.RecordStatement("first")
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	tr.RecordStatement("second")
	tr.Clear()
	tr.Disable()
	snapshot.Restore()
	tr.RecordStatement("third")
	want := "stmt first\n" + traceIndent + "stmt third"
	if got := tr.String(); got != want {
		t.Fatalf("trace after restore:\n%s\nwant:\n%s", got, want)
	}
	snapshot.Release()
}

// The round trips above snapshot every step of these cases, so they must reach a
// queued composite completion and a deferral holding back a sibling's transition.
func TestSnapshotStepsReachAPendingCompositeCompletionAndAHeldDeferral(t *testing.T) {
	conformanceDir := filepath.Join("testdata", "conformance")
	reaches := func(t *testing.T, testName string, at func(*StateExecutor) bool) {
		t.Helper()
		run := newSteppedRun(t, conformanceDir, testName, loadExpectedOutcome(t, conformanceDir, testName))
		for more := true; more && run.taken < maxSnapshotSteps; {
			for _, exec := range run.states {
				if at(exec) {
					return
				}
			}
			var err error
			if more, err = run.advance(); err != nil {
				t.Fatalf("step %d: %v", run.taken, err)
			}
		}
		t.Fatalf("no step boundary of %s shows the state the round trips must carry", testName)
	}

	t.Run("composite_completion_pending", func(t *testing.T) {
		reaches(t, "state_composite_completion_then_machine_done", func(exec *StateExecutor) bool {
			for _, event := range exec.eventQueue.events {
				trans, ok := event.Payload.(*lower.Transition)
				if ok && trans.Trigger == nil && getNodeName(trans.Source) == "s1" && exec.stateComplete(trans.Source.(*ast.StateNode)) {
					return true
				}
			}
			return false
		})
	})
	t.Run("deferral_held_over_a_sibling_transition", func(t *testing.T) {
		reaches(t, "state_deferral_outranks_sibling_region", func(exec *StateExecutor) bool {
			if len(exec.deferred) != 1 {
				return false
			}
			if msg, ok := exec.deferred[0].Payload.(Message); !ok || msg.SignalType != "Ping" {
				return false
			}
			candidates, err := exec.selectCandidates(func(source *ast.StateNode) ([]int, []RunNote, error) {
				return exec.enabledTransitions(source, &exec.deferred[0])
			})
			return err == nil && len(candidates) == 1 && getNodeName(candidates[0].source) == "idle"
		})
	})
}

func TestSnapshotRefusesMidRun(t *testing.T) {
	resolver := resolve.New(symbols.NewIndex())
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10)
	ctx.runDepth++
	if _, err := ctx.Snapshot(); !errors.Is(err, ErrSnapshotMidRun) {
		t.Fatalf("snapshot inside a run: got %v, want ErrSnapshotMidRun", err)
	}
}

func outcomeDiff(want, got roundTripOutcome) string {
	var b strings.Builder
	if want.err != got.err {
		fmt.Fprintf(&b, "error: want %q, got %q\n", want.err, got.err)
	}
	if want.trace != got.trace {
		fmt.Fprintf(&b, "trace differs at line %d\n=== WANT ===\n%s\n=== GOT ===\n%s\n", firstDifferingLine(want.trace, got.trace), want.trace, got.trace)
	}
	if want.values != got.values {
		fmt.Fprintf(&b, "values differ at line %d\n=== WANT ===\n%s\n=== GOT ===\n%s\n", firstDifferingLine(want.values, got.values), want.values, got.values)
	}
	return b.String()
}

func firstDifferingLine(a, b string) int {
	as, bs := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range min(len(as), len(bs)) {
		if as[i] != bs[i] {
			return i + 1
		}
	}
	return min(len(as), len(bs)) + 1
}
