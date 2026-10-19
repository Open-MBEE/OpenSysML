package runtime

import (
	"errors"
	"strings"
	"testing"
)

// An action the object performs under a succession order (`first earlier then
// later;`) has no performance while its predecessor runs: running it on the object
// is refused with the order's violation rather than performed anew beside the
// held one; once the clock has released it, the object's own performance is the
// one found, so the object is written once.
func TestActionHeldBySuccessionOnItsPerformer(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package H {
		private import ScalarValues::*;
		part def Pair {
			attribute marks : Integer = 0;
			perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
			perform action later { first start; then action mark assign marks := marks + 1; then done; }
			first earlier then later;
		}
	}`))
	pair := mustInstantiate(t, ctx, oneSymbol(t, idx, "H::Pair"))
	later := oneSymbol(t, idx, "H::Pair::later")
	if _, err := ctx.ExecuteActionPerformedBy(later, pair, nil); !errors.Is(err, errSuccessionOrderViolated) {
		t.Fatalf("later on the pair while earlier runs: %v, want %v", err, errSuccessionOrderViolated)
	}
	if _, _, err := ctx.StartActionPerformedBy(later, pair, nil); !errors.Is(err, errSuccessionOrderViolated) {
		t.Fatalf("starting later on the pair while earlier runs: %v, want %v", err, errSuccessionOrderViolated)
	}
	if got := featureInt(t, ctx, pair, "marks"); got != 0 {
		t.Fatalf("marks = %d after the refusals, want 0: nothing ran", got)
	}
	if _, err := ctx.Advance(10); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	exec, reused, err := ctx.StartActionPerformedBy(later, pair, nil)
	if err != nil || !reused || !exec.State().Ended() {
		t.Fatalf("later on the pair after the release: exec %v, reused %v, err %v; want the pair's own completed performance", exec, reused, err)
	}
	if _, err := ctx.ExecuteActionPerformedBy(later, pair, nil); err != nil {
		t.Fatalf("later on the pair after the release: %v", err)
	}
	if got := featureInt(t, ctx, pair, "marks"); got != 1 {
		t.Errorf("marks = %d, want 1: the released performance ran once", got)
	}
}

// A held action whose predecessor is itself held (`first earlier then middle;
// first middle then later;`) is refused too: the order admits a start only once
// every predecessor ended, not merely while none has begun. Inputs for a held
// action are refused before anything is released, so the refusal leaves it held.
func TestActionHeldBehindAHeldPredecessorStaysHeld(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package H {
		private import ScalarValues::*;
		part def Chain {
			attribute marks : Integer = 0;
			perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
			perform action middle { first start; then action mark assign marks := marks + 10; then done; }
			perform action later { first start; then action mark assign marks := marks + 1; then done; }
			first earlier then middle;
			first middle then later;
		}
	}`))
	chain := mustInstantiate(t, ctx, oneSymbol(t, idx, "H::Chain"))
	later := oneSymbol(t, idx, "H::Chain::later")
	held := chain.PerformedActionsOf(later)
	if len(held) != 1 || held[0].deferred == nil {
		t.Fatalf("later on the chain: %d performances, want one held back", len(held))
	}
	_, _, err := ctx.StartActionPerformedBy(later, chain, nil)
	if !errors.Is(err, errSuccessionOrderViolated) || !strings.Contains(err.Error(), "has not ended") {
		t.Fatalf("later behind a held middle: %v, want %v naming the predecessor not ended", err, errSuccessionOrderViolated)
	}
	if _, _, err := ctx.StartActionPerformedBy(later, chain, map[string]Value{"n": intArgument(3)}); !errors.Is(err, errPerformedInputs) {
		t.Fatalf("later with inputs: %v, want %v", err, errPerformedInputs)
	}
	if held[0].deferred == nil {
		t.Fatal("a refused start released the held action")
	}
	if _, err := ctx.Advance(10); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if got := featureInt(t, ctx, chain, "marks"); got != 11 {
		t.Errorf("marks = %d, want 11: middle then later ran once each, in order", got)
	}
}

// A held action whose release fails on an unbound input reports that failure
// rather than the executor the failed performance left behind.
func TestActionHeldWhoseReleaseFailsReportsTheFailure(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package H {
		private import ScalarValues::*;
		action def Needy { in n : Integer[1]; first start; then done; }
		part def Pair {
			perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
			perform action later { first start; then action need : Needy; then done; }
			first earlier then later;
		}
	}`))
	pair := mustInstantiate(t, ctx, oneSymbol(t, idx, "H::Pair"))
	if _, err := ctx.Advance(10); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	later := oneSymbol(t, idx, "H::Pair::later")
	if _, _, err := ctx.StartActionPerformedBy(later, pair, nil); !errors.Is(err, errUnboundParameter) {
		t.Fatalf("later whose release failed: %v, want %v", err, errUnboundParameter)
	}
}

// A held action whose declaration binds an `in ref` parameter (`:>> context =
// other`) accepts that binding as an input before it starts, as the started
// performance does: the feature value is the model's, not the start's. The
// check neither releases the held action nor refuses the binding it states.
func TestActionHeldAcceptsItsDeclaredReferenceBinding(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package H {
		private import ScalarValues::*;
		part def Heater { attribute level : Integer = 0; }
		action def Nudge {
			in delta : Integer[1] = 1;
			in ref context : Heater[1];
			first apply;
			action apply { assign context.level := context.level + delta; }
			first apply then done;
		}
		part def Host {
			part other : Heater;
			perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
			perform action later : Nudge { in ref :>> context = other; }
			first earlier then later;
		}
	}`))
	host := mustInstantiate(t, ctx, oneSymbol(t, idx, "H::Host"))
	later := oneSymbol(t, idx, "H::Host::later")
	held := host.PerformedActionsOf(later)
	if len(held) != 1 || held[0].deferred == nil {
		t.Fatalf("later on the host: %d performances, want one held back", len(held))
	}
	fv, err := host.GetFeatureValue(ctx, "other")
	if err != nil {
		t.Fatalf("read other: %v", err)
	}
	other := fv.HeldValue()
	self := Value{Kind: ValInstance, Instance: host.ID}

	_, _, err = ctx.StartActionPerformedBy(later, host, map[string]Value{"context": other})
	if !errors.Is(err, errSuccessionOrderViolated) || errors.Is(err, errPerformedInputs) {
		t.Fatalf("held later with its declared binding: %v, want only %v", err, errSuccessionOrderViolated)
	}
	if _, _, err := ctx.StartActionPerformedBy(later, host, map[string]Value{"context": self}); !errors.Is(err, errPerformedInputs) {
		t.Fatalf("held later offered its performer where `other` is bound: %v, want %v", err, errPerformedInputs)
	}
	if held[0].deferred == nil || held[0].Action != nil {
		t.Fatal("checking the inputs of a held action released it")
	}

	if _, err := ctx.Advance(10); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	otherInst, ok := ctx.Instance(other.Instance)
	if !ok {
		t.Fatalf("other #%d is not an instance", other.Instance)
	}
	if got := featureInt(t, ctx, otherInst, "level"); got != 1 {
		t.Errorf("other.level = %d, want 1: later ran once when earlier ended", got)
	}
	if _, err := ctx.ExecuteActionPerformedBy(later, host, map[string]Value{"context": other}); err != nil {
		t.Fatalf("started later with its declared binding: %v", err)
	}
	if _, err := ctx.ExecuteActionPerformedBy(later, host, map[string]Value{"context": self}); !errors.Is(err, errPerformedInputs) {
		t.Fatalf("started later offered its performer: %v, want %v", err, errPerformedInputs)
	}
	if got := featureInt(t, ctx, otherInst, "level"); got != 1 {
		t.Errorf("other.level = %d, want 1: joining the performance does not run it again", got)
	}
}

// Checking the inputs of a held action against its declared binding evaluates the
// binding as a probe: a refused start materializes no part, starts no behavior
// and writes nothing, even where the bound part is still unmaterialized and its
// type performs an action of its own.
func TestActionHeldInputCheckLeavesTheModelUnchanged(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package H {
		private import ScalarValues::*;
		part def Counter {
			attribute level : Integer = 0;
			attribute bumps : Integer = 0;
			perform action bump { first start; then action m assign bumps := bumps + 1; then done; }
		}
		action def Nudge {
			in delta : Integer[1] = 1;
			in ref context : Counter[1];
			first apply;
			action apply { assign context.level := context.level + delta; }
			first apply then done;
		}
		part def Host {
			part other : Counter;
			perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
			perform action later : Nudge { in ref :>> context = other; }
			first earlier then later;
		}
	}`))
	host := mustInstantiate(t, ctx, oneSymbol(t, idx, "H::Host"))
	later := oneSymbol(t, idx, "H::Host::later")
	held := host.PerformedActionsOf(later)
	if len(held) != 1 || held[0].deferred == nil {
		t.Fatalf("later on the host: %d performances, want one held back", len(held))
	}
	objects, behaviors := len(ctx.created), len(ctx.objectBehaviors)

	self := Value{Kind: ValInstance, Instance: host.ID}
	if _, _, err := ctx.StartActionPerformedBy(later, host, map[string]Value{"context": self}); !errors.Is(err, errPerformedInputs) {
		t.Fatalf("held later offered its performer where `other` is bound: %v, want %v", err, errPerformedInputs)
	}
	if len(ctx.created) != objects || len(ctx.objectBehaviors) != behaviors {
		t.Fatalf("a refused start left %d object(s) and %d behavior(s) behind",
			len(ctx.created)-objects, len(ctx.objectBehaviors)-behaviors)
	}
	if held[0].deferred == nil || held[0].Action != nil {
		t.Fatal("a refused start released the held action")
	}

	if _, err := ctx.Advance(10); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	fv, err := host.GetFeatureValue(ctx, "other")
	if err != nil {
		t.Fatalf("read other: %v", err)
	}
	other, ok := ctx.Instance(fv.HeldValue().Instance)
	if !ok {
		t.Fatalf("other is not an object: %v", fv.HeldValue())
	}
	if got := featureInt(t, ctx, other, "level"); got != 1 {
		t.Errorf("other.level = %d, want 1: later ran once when earlier ended", got)
	}
	if got := featureInt(t, ctx, other, "bumps"); got != 1 {
		t.Errorf("other.bumps = %d, want 1: other's own behavior ran once, on materialization", got)
	}
}

// A declared binding that fails to evaluate is reported as the release would
// report it, rather than refusing the input; another argument's failure does not
// touch the comparison of the one offered.
func TestActionHeldReportsAFailingDeclaredBinding(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package H {
		private import ScalarValues::*;
		part def Heater { attribute level : Integer = 0; }
		action def Nudge {
			in delta : Integer[1] default = 1;
			in ref context : Heater[1];
			first apply;
			action apply { assign context.level := context.level + delta; }
			first apply then done;
		}
		part def Host {
			attribute zero : Integer = 0;
			ref pick : Heater[1];
			part other : Heater;
			perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
			perform action unpicked : Nudge { in ref :>> context = pick; }
			perform action undivided : Nudge { in ref :>> context = other; in :>> delta = 1 / zero; }
			first earlier then unpicked;
			first earlier then undivided;
		}
	}`))
	host := mustInstantiate(t, ctx, oneSymbol(t, idx, "H::Host"))
	fv, err := host.GetFeatureValue(ctx, "other")
	if err != nil {
		t.Fatalf("read other: %v", err)
	}
	other := fv.HeldValue()
	self := Value{Kind: ValInstance, Instance: host.ID}

	unpicked := oneSymbol(t, idx, "H::Host::unpicked")
	_, _, err = ctx.StartActionPerformedBy(unpicked, host, map[string]Value{"context": other})
	if !errors.Is(err, errUninitializedFeatureValue) || !strings.Contains(err.Error(), "bind context") || errors.Is(err, errPerformedInputs) {
		t.Fatalf("held unpicked whose binding fails: %v, want the binding's failure, not %v", err, errPerformedInputs)
	}
	if held := host.PerformedActionsOf(unpicked); len(held) != 1 || held[0].deferred == nil {
		t.Fatal("a failed binding check released the held action")
	}

	undivided := oneSymbol(t, idx, "H::Host::undivided")
	_, _, err = ctx.StartActionPerformedBy(undivided, host, map[string]Value{"context": other})
	if !errors.Is(err, errSuccessionOrderViolated) || errors.Is(err, errPerformedInputs) {
		t.Fatalf("held undivided offered its declared binding: %v, want only %v", err, errSuccessionOrderViolated)
	}
	if _, _, err := ctx.StartActionPerformedBy(undivided, host, map[string]Value{"context": self}); !errors.Is(err, errPerformedInputs) {
		t.Fatalf("held undivided offered its performer: %v, want %v", err, errPerformedInputs)
	}
}

// A declared binding read through a calc usage is evaluated anew by every input
// check: what one check computed is not held past it, so a check after a change to
// what the usage reads compares against the usage's current answer.
func TestActionHeldBindingThroughACalcUsageReadsItAnew(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package H {
		private import ScalarValues::*;
		part def Heater { attribute level : Integer = 0; }
		calc def Pick { in useLeft : Boolean; in left : Heater; in right : Heater; return : Heater = if useLeft ? left else right; }
		action def Nudge {
			in delta : Integer[1] = 1;
			in ref context : Heater[1];
			first apply;
			action apply { assign context.level := context.level + delta; }
			first apply then done;
		}
		part def Host {
			attribute useLeft : Boolean = true;
			part left : Heater;
			part right : Heater;
			calc pick : Pick { in useLeft = useLeft; in left = left; in right = right; }
			perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
			perform action later : Nudge { in ref :>> context = pick; }
			first earlier then later;
		}
	}`))
	host := mustInstantiate(t, ctx, oneSymbol(t, idx, "H::Host"))
	later := oneSymbol(t, idx, "H::Host::later")
	side := func(name string) Value {
		fv, err := host.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return fv.HeldValue()
	}
	left, right := side("left"), side("right")
	if n := len(ctx.run.calcUsageRuns); n != 0 {
		t.Fatalf("%d activation(s) hold a calc usage evaluation before any check", n)
	}

	_, _, err := ctx.StartActionPerformedBy(later, host, map[string]Value{"context": left})
	if !errors.Is(err, errSuccessionOrderViolated) || errors.Is(err, errPerformedInputs) {
		t.Fatalf("held later offered left while useLeft: %v, want only %v", err, errSuccessionOrderViolated)
	}
	if _, _, err := ctx.StartActionPerformedBy(later, host, map[string]Value{"context": right}); !errors.Is(err, errPerformedInputs) {
		t.Fatalf("held later offered right while useLeft: %v, want %v", err, errPerformedInputs)
	}
	if n := len(ctx.run.calcUsageRuns); n != 0 {
		t.Errorf("%d activation(s) hold a calc usage evaluation after the checks, want none: a check holds nothing past itself", n)
	}
	if err := host.SetFeatureValue(ctx, "useLeft", boolValue(false)); err != nil {
		t.Fatalf("set useLeft: %v", err)
	}
	if _, _, err := ctx.StartActionPerformedBy(later, host, map[string]Value{"context": left}); !errors.Is(err, errPerformedInputs) {
		t.Fatalf("held later offered left once useLeft is false: %v, want %v", err, errPerformedInputs)
	}
	_, _, err = ctx.StartActionPerformedBy(later, host, map[string]Value{"context": right})
	if !errors.Is(err, errSuccessionOrderViolated) || errors.Is(err, errPerformedInputs) {
		t.Fatalf("held later offered right once useLeft is false: %v, want only %v", err, errSuccessionOrderViolated)
	}
}

// Checking a held action's inputs outside a run has a run's own step budget, as
// the start it precedes would: a budget an earlier run spent does not refuse the
// declared binding.
func TestActionHeldInputCheckHasItsOwnBudget(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package H {
		private import ScalarValues::*;
		part def Heater { attribute level : Integer = 0; }
		action def Nudge {
			in delta : Integer[1] = 1;
			in ref context : Heater[1];
			first apply;
			action apply { assign context.level := context.level + delta; }
			first apply then done;
		}
		part def Host {
			part other : Heater;
			perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
			perform action later : Nudge { in ref :>> context = other; }
			first earlier then later;
		}
	}`))
	host := mustInstantiate(t, ctx, oneSymbol(t, idx, "H::Host"))
	later := oneSymbol(t, idx, "H::Host::later")
	fv, err := host.GetFeatureValue(ctx, "other")
	if err != nil {
		t.Fatalf("read other: %v", err)
	}
	other := fv.HeldValue()

	// The run left behind at the prompt has spent its whole budget.
	ctx.run.steps = ctx.maxSteps
	_, _, err = ctx.StartActionPerformedBy(later, host, map[string]Value{"context": other})
	if errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("held later checked after a spent run: %v, want the check on a budget of its own", err)
	}
	if !errors.Is(err, errSuccessionOrderViolated) || errors.Is(err, errPerformedInputs) {
		t.Fatalf("held later offered its declared binding: %v, want only %v", err, errSuccessionOrderViolated)
	}
}
