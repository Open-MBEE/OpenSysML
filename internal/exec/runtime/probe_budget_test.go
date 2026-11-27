package runtime

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// An evidence probe reads on its own bounded budget; nested probes share it and
// leave the caller's counters untouched even when its steps or elements are spent.
func TestProbeEvidenceHasAnIndependentBudget(t *testing.T) {
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P { part def D; }`))
	ctx.maxSteps = 10
	ctx.maxElements = 2
	ctx.run.steps, ctx.run.elements = 8, 1
	outer := ctx.run
	spend := func(steps int) error {
		for i := 0; i < steps; i++ {
			if err := ctx.incrementStep(); err != nil {
				return err
			}
		}
		return nil
	}

	err := ctx.ProbeEvidence(func() error {
		if err := spend(5); err != nil {
			return err
		}
		return ctx.Probe(func() error { return spend(5) })
	})
	if err != nil {
		t.Fatalf("evidence preview after a nearly spent budget: %v", err)
	}
	err = ctx.ProbeEvidence(func() error { return ctx.chargeElements(2) })
	if err != nil {
		t.Fatalf("evidence preview within its fresh element budget: %v", err)
	}
	if ctx.run != outer || outer.steps != 8 || outer.elements != 1 {
		t.Fatalf("the probes left run %v with %d steps and %d elements, want the caller's %d and %d",
			ctx.run == outer, outer.steps, outer.elements, 8, 1)
	}
	if err := spend(3); !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("an ordinary run after the probe = %v, want %v", err, ErrStepLimitExceeded)
	}
	if err := ctx.chargeElements(5); !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("an ordinary element charge after the probe = %v, want %v", err, ErrElementLimitExceeded)
	}
}

// Bulk compiled charges and required-tail listings stay inside a preview's fresh
// limits; the caller's guards return once the probe ends.
func TestProbeEvidenceBoundsBulkStepAndElementGuards(t *testing.T) {
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P { part def D; }`))
	ctx.maxSteps, ctx.maxElements = 6, 2
	ctx.run.steps = 5
	huge := NewSequenceValue(&Sequence{required: &requiredMembers{ctx: ctx, count: 10}})
	small := NewSequenceValue(&Sequence{required: &requiredMembers{ctx: ctx, count: 2}})

	if err := ctx.ProbeEvidence(func() error { return ctx.chargeSteps(1) }); err != nil {
		t.Fatalf("a bulk charge inside the preview's fresh step budget: %v", err)
	}
	if err := ctx.ProbeEvidence(func() error { return ctx.chargeSteps(20) }); !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("a bulk charge past the preview's step budget = %v, want %v", err, ErrStepLimitExceeded)
	}
	err := ctx.ProbeEvidence(func() error {
		held, err := ctx.HeldElements(small)
		if err != nil {
			return err
		}
		if len(held) != 2 {
			return fmt.Errorf("held elements = %d, want 2", len(held))
		}
		_, err = ctx.HeldElements(huge)
		return err
	})
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("a listing past the preview's element budget = %v, want %v", err, ErrElementLimitExceeded)
	}
	if ctx.run.steps != 5 {
		t.Fatalf("the caller's steps = %d, want 5", ctx.run.steps)
	}
	if err := ctx.chargeSteps(20); !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("an ordinary bulk charge after the probe = %v, want %v", err, ErrStepLimitExceeded)
	}
	if _, err := ctx.HeldElements(huge); !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("an ordinary listing after the probe = %v, want %v", err, ErrElementLimitExceeded)
	}
}

// A preview that charges steps in bulk — as a compiled default does — must fail
// within its own budget instead of running on without a limit.
func TestEvidenceProbeStopsARunawayCompiledDefault(t *testing.T) {
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P { part def D; }`))
	budgets := ctx.Budgets()
	budgets.MaxSteps = 8
	if err := ctx.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	ctx.run.steps = 1
	err := ctx.ProbeEvidence(func() error {
		for i := 0; i < 100; i++ {
			if err := ctx.chargeSteps(1); err != nil {
				return err
			}
		}
		return nil
	})
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("a runaway preview = %v, want %v", err, ErrStepLimitExceeded)
	}
	if ctx.run.steps != 1 {
		t.Fatalf("the caller's steps after the preview = %d, want 1", ctx.run.steps)
	}
}

// Listing a huge lazy collection inside a preview refuses with the typed element
// error before its members are made, rather than growing the held population.
func TestEvidenceProbeRefusesAHugeLazyCollection(t *testing.T) {
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P { part def D; }`))
	budgets := ctx.Budgets()
	budgets.MaxElements = 3
	if err := ctx.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	before := len(ctx.InstanceIDs())
	huge := NewSequenceValue(&Sequence{required: &requiredMembers{ctx: ctx, count: 1000000}})
	err := ctx.ProbeEvidence(func() error {
		_, err := ctx.HeldElements(huge)
		return err
	})
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("a huge lazy listing in a preview = %v, want %v", err, ErrElementLimitExceeded)
	}
	if got := len(ctx.InstanceIDs()); got != before {
		t.Fatalf("the refused preview made %d objects, want %d", got, before)
	}
}

// A preview continues the caller's seeded stream: it sees the next draw a regular
// read would, and leaves the stream where the caller left it.
func TestEvidenceProbeContinuesTheSeededDrawStream(t *testing.T) {
	m := parseLibraryModel(t, drawingModel)
	ctx, _ := m.fresh()
	mustSchedule(t, ctx, mustPolicy(t, "seed:5"))
	ctx.SetModelSeed(11)
	ctx.run.scheduler = ctx.schedulerUnder(ctx.schedule)
	unit := distribution{
		draw:   func(rng *rand.Rand) semantics.Value { return drawnReal(rng.Float64()) },
		admits: realWithin(0, 1),
	}
	if _, err := ctx.draw("regular", unit); err != nil {
		t.Fatal(err)
	}
	peek := func() semantics.Value {
		t.Helper()
		saved := *ctx.run.scheduler.modeled.pcg
		v, err := ctx.run.scheduler.draw("peek", unit)
		if err != nil {
			t.Fatal(err)
		}
		*ctx.run.scheduler.modeled.pcg = saved
		return v
	}
	next := peek()

	var preview semantics.Value
	if err := ctx.ProbeEvidence(func() error {
		var err error
		preview, err = ctx.draw("evidence", unit)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if preview != next {
		t.Fatalf("the preview drew %v, want the regular read's next draw %v", preview, next)
	}
	if after := peek(); after != next {
		t.Fatalf("the preview moved the seeded stream: next %v, now %v", next, after)
	}
	if draws := ctx.DrawsTaken(); len(draws) != 1 || draws[0].What != "regular" {
		t.Fatalf("a preview draw was recorded: %v", draws)
	}
}

// A probe's budget error names its fixed cap, not a variable that cannot lift it;
// an ordinary run still names the variable that raises its configured budget.
func TestEvidenceBudgetHintNamesAKnobThatChangesIt(t *testing.T) {
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P { part def D; }`))
	ctx.maxSteps, ctx.maxElements = 2*DefaultMaxSteps, 2*DefaultMaxElements
	huge := NewSequenceValue(&Sequence{required: &requiredMembers{ctx: ctx, count: DefaultMaxElements + 1}})
	probes := []error{
		ctx.ProbeEvidence(func() error { return ctx.chargeSteps(DefaultMaxSteps + 1) }),
		ctx.ProbeEvidence(func() error { return ctx.chargeElements(DefaultMaxElements + 1) }),
		ctx.ProbeEvidence(func() error {
			_, err := ctx.HeldElements(huge)
			return err
		}),
	}
	for _, err := range probes {
		if !errors.Is(err, ErrStepLimitExceeded) && !errors.Is(err, ErrElementLimitExceeded) {
			t.Fatalf("a probe past its fixed budget = %v, want a typed budget error", err)
		}
		if !strings.Contains(err.Error(), "evidence preview budget is fixed") ||
			strings.Contains(err.Error(), MaxStepsEnvVar) || strings.Contains(err.Error(), MaxElementsEnvVar) {
			t.Fatalf("a probe's budget error names a variable that cannot lift its cap: %v", err)
		}
	}
	ctx.run.steps, ctx.run.elements = ctx.maxSteps, ctx.maxElements
	if err := ctx.chargeSteps(1); !errors.Is(err, ErrStepLimitExceeded) || !strings.Contains(err.Error(), MaxStepsEnvVar) {
		t.Fatalf("an ordinary step error = %v, want one naming %s", err, MaxStepsEnvVar)
	}
	if err := ctx.chargeElements(1); !errors.Is(err, ErrElementLimitExceeded) || !strings.Contains(err.Error(), MaxElementsEnvVar) {
		t.Fatalf("an ordinary element error = %v, want one naming %s", err, MaxElementsEnvVar)
	}
}

// Cancellation still reaches an evidence read: an independent budget is not an
// exemption from the driver's interrupt.
func TestProbeEvidenceHonoursTheInterrupt(t *testing.T) {
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P { part def D; }`))
	flag := &atomic.Bool{}
	ctx.SetInterrupt(flag)
	flag.Store(true)
	if err := ctx.ProbeEvidence(func() error { return ctx.incrementStep() }); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("interrupted evidence preview = %v, want %v", err, ErrInterrupted)
	}
}

// An evidence probe still journals what it changes: a written feature value and
// an object materialized for a lazy part must not outlive the probe.
func TestProbeEvidenceRollsBackWritesAndObjects(t *testing.T) {
	src := `package P {
		part def Child { attribute v : Integer = 2; }
		part def D { attribute v : Integer = 1; part child : Child[1]; }
		part d : D;
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	obj := instantiateNamed(t, ctx, idx, "P::d")
	if got := readInt(t, ctx, obj, "v"); got != 1 {
		t.Fatalf("v before the probe = %d, want 1", got)
	}
	held := len(ctx.InstanceIDs())
	lazy := obj.FeatureValues["child"]
	if lazy == nil || lazy.Materialized {
		t.Fatalf("child before the probe = %+v, want an unmaterialized part", lazy)
	}

	err := ctx.ProbeEvidence(func() error {
		setInt(t, ctx, obj, "v", 7)
		if got := readInt(t, ctx, obj, "v"); got != 7 {
			return fmt.Errorf("v in the probe = %d, want 7", got)
		}
		child, err := obj.GetFeatureValue(ctx, "child")
		if err != nil {
			return err
		}
		if child == nil || !lazy.Materialized {
			return fmt.Errorf("the probe did not materialize the child")
		}
		if len(ctx.InstanceIDs()) <= held {
			return fmt.Errorf("the probe materialized no new object: %v", ctx.InstanceIDs())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("evidence preview: %v", err)
	}
	if got := readInt(t, ctx, obj, "v"); got != 1 {
		t.Fatalf("v after the probe = %d, want 1", got)
	}
	if len(ctx.InstanceIDs()) != held {
		t.Fatalf("the probe left %d objects, want %d", len(ctx.InstanceIDs()), held)
	}
	if lazy.Materialized {
		t.Fatal("the probe left the lazy child materialized")
	}
}

// A probe that allocated no identity skips the identity scan on rollback, so a
// population of reads over held objects stays linear under the runtime lock.
func TestProbeWithoutAnAllocationDoesNotScanHeldIdentities(t *testing.T) {
	src := `package P { part def D { attribute v : Integer = 1; }`
	for i := 0; i < 3000; i++ {
		src += fmt.Sprintf(" part p%d : D;", i)
	}
	src += " }"
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	for i := 0; i < 3000; i++ {
		instantiateNamed(t, ctx, idx, fmt.Sprintf("P::p%d", i))
	}
	const probes = 3000
	start := time.Now()
	for i := 0; i < probes; i++ {
		if err := ctx.Probe(func() error { return nil }); err != nil {
			t.Fatalf("probe %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	// Each rollback used to walk all 3000 held identities: nine million visits
	// here. The fast path does none, leaving a wide margin for slower machines.
	if elapsed > 20*time.Millisecond {
		t.Fatalf("%d probes over 3000 held objects took %s, want well under 20ms", probes, elapsed)
	}
}
