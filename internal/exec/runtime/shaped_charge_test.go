package runtime

import (
	"errors"
	"testing"
)

// TestHeldResultIsChargedOnceAFeatureHoldsIt pins when the one element of a held
// result costs: not when the result is shaped, once when a feature holds it, and
// a charge the budget refuses leaves the shape still owing its element.
func TestHeldResultIsChargedOnceAFeatureHoldsIt(t *testing.T) {
	ec, pair := collectionExprContext(t, "(7, 8)", DefaultMaxSteps)
	ctx := ec.ctx
	evaluated, err := ec.Eval(pair)
	if err != nil {
		t.Fatal(err)
	}
	held := ctx.collectionOf(&EffectiveFeature{}, elementsOf(evaluated)[:1])
	markShaped(held)
	if !held.Sequence().shaped {
		t.Fatal("a held result is not marked as owing its element")
	}

	charged := ctx.run.elements
	ctx.maxElements = charged
	if err := ctx.chargeShaped(held); !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("charging past the element budget: got %v, want %v", err, ErrElementLimitExceeded)
	}
	if !held.Sequence().shaped {
		t.Fatal("a refused charge cleared the marker: the element would never be charged")
	}

	ctx.run.elements = charged
	ctx.maxElements = charged + 1
	if err := ctx.chargeShaped(held); err != nil {
		t.Fatalf("charging within the element budget: %v", err)
	}
	if held.Sequence().shaped {
		t.Fatal("a charged shape still owes its element")
	}
	if got := ctx.run.elements; got != charged+1 {
		t.Fatalf("holding the result charged %d elements, want 1", got-charged)
	}
	if err := ctx.chargeShaped(held); err != nil || ctx.run.elements != charged+1 {
		t.Fatalf("holding the result again: err %v, elements %d, want no further charge", err, ctx.run.elements-charged)
	}
}
