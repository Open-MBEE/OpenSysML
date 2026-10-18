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
