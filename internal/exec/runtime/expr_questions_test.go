package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

const exprQuestionsSrc = `
	package P {
		part def A {
			attribute x = 1;
			attribute folded = 2 * 3 + 1;
			attribute open = x + 1;
		}
		part def B :> A { attribute redefines folded; }
	}
`

// TestLoweredRecordAnswersTheSemanticQuestions pins that a lowered record answers the fold
// its syntax has, in the scope the caller names, and that no record answers nothing.
func TestLoweredRecordAnswersTheSemanticQuestions(t *testing.T) {
	ctx, idx := contextForSource(t, exprQuestionsSrc)
	sem, scope := ctx.model.semantics, lookupOne(t, idx, "P::A").Scope
	folded := ctx.model.loweredExpr(scope, loweredValue(t, idx, "P::A::folded"))
	if got, ok := folded.entry.Folded(sem, ctx.maxIntegerBits); !ok || got.Int != 7 {
		t.Fatalf("2 * 3 + 1 folds to %v, %v; want 7", got, ok)
	}
	open := ctx.model.loweredExpr(scope, loweredValue(t, idx, "P::A::open"))
	if got, ok := open.entry.Folded(sem, ctx.maxIntegerBits); ok {
		t.Fatalf("x + 1 folds to %v", got)
	}
	if count, ok := open.entry.Held(sem, scope); ok != func() bool { _, want := sem.ValuesHeldBy(scope, open.tree()); return want }() {
		t.Fatalf("held = %v, %v; differs from the syntax's", count, ok)
	}

	var none *lower.ExprEntry
	if _, ok := none.Folded(sem, ctx.maxIntegerBits); ok {
		t.Fatal("no record folded")
	}
	if _, ok := none.Dimension(sem, scope); ok {
		t.Fatal("no record has a dimension")
	}
	if _, ok := none.Held(sem, scope); ok {
		t.Fatal("no record holds a count")
	}
	if _, err := none.Classifies(sem, scope, nil); err == nil {
		t.Fatal("no record classified")
	}
	if _, err := none.Unit(sem, scope); err == nil {
		t.Fatal("no record read as a unit")
	}
}

// TestLoweredDefaultIsTheOneAFeatureStatesOrRedefines pins that a default taken from a
// redefined feature is lowered where that feature wrote it, and that none is reported.
func TestLoweredDefaultIsTheOneAFeatureStatesOrRedefines(t *testing.T) {
	ctx, idx := contextForSource(t, exprQuestionsSrc)
	a, b := lookupOne(t, idx, "P::A"), lookupOne(t, idx, "P::B")
	written := loweredValue(t, idx, "P::A::folded")
	own, ok := ctx.loweredDefault(lookupOne(t, idx, "P::A::folded"), a)
	if !ok || own.tree() != written || own.entry.Scope != a.Scope {
		t.Fatalf("own default = %v, %v", own.entry, ok)
	}
	taken, ok := ctx.loweredDefault(lookupOne(t, idx, "P::B::folded"), b)
	if !ok || taken.entry != own.entry {
		t.Fatalf("redefined default = %v, %v; want the record of A's", taken.entry, ok)
	}
	if got, ok := ctx.constantIntegers(lookupOne(t, idx, "P::B::folded"), b); !ok || len(got) != 1 || got[0] != 7 {
		t.Fatalf("constant integers = %v, %v; want [7]", got, ok)
	}
	if _, ok := ctx.loweredDefault(b, a); ok {
		t.Fatal("a definition has a default")
	}
}
