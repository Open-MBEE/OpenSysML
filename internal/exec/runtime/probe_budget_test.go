package runtime

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// An evidence probe reads on its own budget; nested probes share it and leave
// the caller's counters untouched even when its steps or elements are spent.
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
	err = ctx.ProbeEvidence(func() error { return ctx.chargeElements(5) })
	if err != nil {
		t.Fatalf("evidence preview past the element budget: %v", err)
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

// Bulk compiled charges and required-tail listings bypass their budgets in an
// evidence preview too; the caller's guards return once the probe ends.
func TestProbeEvidenceIgnoresBulkStepAndElementGuards(t *testing.T) {
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P { part def D; }`))
	ctx.maxSteps, ctx.maxElements = 6, 2
	ctx.run.steps = 5
	sequence := NewSequenceValue(&Sequence{required: &requiredMembers{ctx: ctx, count: 10}})

	err := ctx.ProbeEvidence(func() error {
		if err := ctx.chargeSteps(20); err != nil {
			return err
		}
		held, err := ctx.HeldElements(sequence)
		if err != nil {
			return err
		}
		if len(held) != 10 {
			return fmt.Errorf("held elements = %d, want 10", len(held))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("evidence preview past both bulk budgets: %v", err)
	}
	if ctx.run.steps != 5 {
		t.Fatalf("the caller's steps = %d, want 5", ctx.run.steps)
	}
	if err := ctx.chargeSteps(20); !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("an ordinary bulk charge after the probe = %v, want %v", err, ErrStepLimitExceeded)
	}
	if _, err := ctx.HeldElements(sequence); !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("an ordinary listing after the probe = %v, want %v", err, ErrElementLimitExceeded)
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
