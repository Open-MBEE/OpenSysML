package runtime

import (
	"errors"
	"testing"
)

// TestSuccessionCycleMemo: the none-held memo answers the cycle check only while
// no behavior has entered held; one entering, by instantiation or by a snapshot
// restore, sends the check back to the walk, whose verdict it must then match.
func TestSuccessionCycleMemo(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		part def Plain {
			attribute n : Integer = 0;
			perform action step { assign n := 1; }
		}
		part def Ring {
			perform action a { action stepA; }
			perform action b { action stepB; }
			first a then b;
			first b then a;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	if _, err := ctx.Instantiate(oneSymbol(t, idx, "test::Plain")); err != nil {
		t.Fatalf("instantiate Plain: %v", err)
	}
	requireSameCycleVerdict(t, ctx)
	if !ctx.noneHeld.answers(ctx) {
		t.Fatal("cycle check over no held behavior left no memo")
	}

	_, cached := ctx.Instantiate(oneSymbol(t, idx, "test::Ring"))
	requireSuccessionError(t, cached, ErrSuccessionOrderCycle, successionOrderCycleCode)
	if ctx.noneHeld.answers(ctx) {
		t.Fatal("memo still answers after behaviors entered held")
	}
	requireSameCycleVerdict(t, ctx)

	// The same objects in a context whose check never answered from the memo.
	freshIdx, _, fresh := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	if _, err := fresh.Instantiate(oneSymbol(t, freshIdx, "test::Plain")); err != nil {
		t.Fatalf("instantiate Plain afresh: %v", err)
	}
	fresh.noneHeld = noneHeldMemo{}
	_, uncached := fresh.Instantiate(oneSymbol(t, freshIdx, "test::Ring"))
	if uncached == nil || uncached.Error() != cached.Error() {
		t.Fatalf("cycle after memo = %v, want the uncached %v", cached, uncached)
	}

	timed, inst := instantiateTimedSuccession(t)
	snapshot, err := timed.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if _, err := timed.Advance(1); err != nil {
		t.Fatalf("advance clock: %v", err)
	}
	if behaviorNamed(t, inst, "later").deferred != nil {
		t.Fatal("Advance did not release later")
	}
	requireSameCycleVerdict(t, timed)
	if !timed.noneHeld.answers(timed) {
		t.Fatal("cycle check over released behaviors left no memo")
	}
	snapshot.Restore()
	if behaviorNamed(t, inst, "later").deferred == nil {
		t.Fatal("restore did not return later to held")
	}
	if timed.noneHeld.answers(timed) {
		t.Fatal("memo still answers after a restore returned a behavior to held")
	}
	requireSameCycleVerdict(t, timed)
}

func requireSameCycleVerdict(t *testing.T, ctx *Context) {
	t.Helper()
	got, want := ctx.successionCycle(), ctx.walkSuccessionCycle()
	if (got == nil) != (want == nil) || (got != nil && got.Error() != want.Error()) {
		t.Fatalf("memoized cycle check = %v, want the walk's %v", got, want)
	}
	if !errors.Is(got, ErrSuccessionOrderCycle) && want != nil {
		t.Fatalf("memoized cycle check lost the sentinel: %v", got)
	}
}
