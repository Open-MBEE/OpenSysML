package runtime

import (
	"errors"
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
