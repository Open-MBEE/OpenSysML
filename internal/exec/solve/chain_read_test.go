package solve

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// nestedReadSource declares a part whose attributes hold nothing, declare a
// value, declare one that does not evaluate, or declare one reading a feature
// with no value — and constraints reading them through a feature chain never,
// on every assignment, or only under some assignments of a free feature.
const nestedReadSource = `package test {
	private import ScalarValues::*;
	part def Inner {
		attribute input : Real;
		attribute dependent : Real = input + 1.0;
		attribute broken : Real = 1.0 / 0.0;
		attribute fine : Real = 2.0;
		attribute free : Real;
	}
	part def Item {
		part inner : Inner;
		attribute gate : Real;
		assert constraint direct { inner.dependent > 0.0 }
		assert constraint always { false or inner.dependent > 0.0 }
		assert constraint never { true or inner.broken > 0.0 }
		assert constraint gated { gate > 0.0 or inner.dependent > 0.0 }
		assert constraint gatedBroken { gate > 0.0 or inner.broken > 0.0 }
		assert constraint fine { inner.fine > 0.0 }
		assert constraint unbound { inner.free > 0.0 }
	}
	part item : Item;
}`

// chainGuarded reads the chains of both queries as the verify service does:
// the values ChainPins reads are fixed, and the reads it cannot make are
// guarded or refused by UnfixedRead together with the direct ones.
func chainGuarded(t *testing.T, g guardedQueries) (guardedQueries, error) {
	t.Helper()
	for _, q := range []**Query{&g.question, &g.violation} {
		if err := UnfixedRead(g.ctx, *q, g.unfixed); err != nil {
			return g, err
		}
		chainPins, chainUnfixed := ChainPins(g.ctx, *q, ObjectReader(g.ctx, nil), nil)
		if len(chainPins) != 0 {
			t.Fatalf("%s: chain pins %+v, want the unreadable and unbound chains left unpinned", g.sym.Name, chainPins)
		}
		if err := UnfixedRead(g.ctx, *q, chainUnfixed); err != nil {
			return g, err
		}
	}
	return g, nil
}

// TestChainPinsReportAFailedDeclaredValue: a chain reaching a feature whose
// declared value does not evaluate — by division by zero or by reading a
// feature with no value — is an Unfixed naming its exact variable, not a value
// skipped: UnfixedRead then refuses a read on every path and guards a
// conditional one, keeping the variable out of Free. A chain reaching a feature
// that holds nothing stays free, and one reaching a value is pinned to it.
func TestChainPinsReportAFailedDeclaredValue(t *testing.T) {
	ctx, idx := fixture(t, "nested_read.sysml", nestedReadSource)
	dependent := symbolNamed(t, idx, "test::Inner::dependent")

	direct := guardedItemQueries(t, ctx, idx, "direct")
	for _, u := range direct.unfixed {
		if u.Feature == dependent {
			t.Fatalf("FixedFor direct reports %+v, want the chain left to ChainPins", u)
		}
	}
	for _, p := range direct.pins {
		if p.Feature == dependent {
			t.Fatalf("FixedFor direct pins %+v, want the chain left to ChainPins", p)
		}
	}
	pins, unfixed := ChainPins(ctx, direct.question, ObjectReader(ctx, nil), nil)
	if len(pins) != 0 {
		t.Errorf("ChainPins direct pinned %+v, want nothing", pins)
	}
	if len(unfixed) != 1 || unfixed[0].Var != "test::Item::inner.dependent" || unfixed[0].Name != unfixed[0].Var ||
		unfixed[0].Feature != dependent || !errors.Is(unfixed[0].Err, runtime.ErrNoValue) ||
		!errors.Is(unfixed[0].Err, runtime.ErrFeatureValueMaterialization) || unfixed[0].Reason != unfixed[0].Err.Error() {
		t.Fatalf("ChainPins direct unfixed = %+v, want inner.dependent by a missing value", unfixed)
	}
	err := UnfixedRead(ctx, direct.question, unfixed)
	if err == nil || !errors.Is(err, runtime.ErrNoValue) ||
		!strings.HasPrefix(err.Error(), "constraint direct reads test::Item::inner.dependent, whose value could not be read: ") ||
		!strings.HasSuffix(err.Error(), "no value for feature input") {
		t.Errorf("UnfixedRead direct = %v, want the read on every path refused", err)
	}
	if _, err := ctx.CheckConstraintOn(direct.sym, direct.sym.OwnerScope, nil); !errors.Is(err, runtime.ErrNoValue) {
		t.Errorf("evaluate direct = %v, want the evaluator failing on the same read", err)
	}

	always, err := chainGuarded(t, guardedItemQueries(t, ctx, idx, "always"))
	if err != nil {
		t.Fatalf("always = %v, want the read under (not false) guarded", err)
	}
	if v := varNamed(t, always.question, "test::Item::inner.dependent"); len(always.question.Unreadable) != 1 ||
		always.question.Unreadable[0].Var != v || v.Reached == nil {
		t.Errorf("always Unreadable = %+v, want inner.dependent under its read condition", always.question.Unreadable)
	}

	gated, err := chainGuarded(t, guardedItemQueries(t, ctx, idx, "gatedBroken"))
	if err != nil {
		t.Fatalf("gatedBroken = %v, want the read under gate <= 0.0 guarded", err)
	}
	broken := varNamed(t, gated.question, "test::Item::inner.broken")
	if len(gated.question.Unreadable) != 1 || gated.question.Unreadable[0].Var != broken ||
		!errors.Is(gated.question.Unreadable[0].Err, runtime.ErrDivisionByZero) {
		t.Errorf("gatedBroken Unreadable = %+v, want inner.broken by division by zero", gated.question.Unreadable)
	}
	for _, v := range gated.question.Free() {
		if v == broken {
			t.Errorf("gatedBroken Free() = %v includes the unreadable inner.broken", gated.question.Free())
		}
	}
	if err := gated.question.ReachedError(); !errors.Is(err, runtime.ErrDivisionByZero) ||
		!strings.Contains(err.Error(), "reads test::Item::inner.broken under some assignment") {
		t.Errorf("gatedBroken ReachedError() = %v", err)
	}

	fine := guardedItemQueries(t, ctx, idx, "fine")
	pins, unfixed = ChainPins(ctx, fine.question, ObjectReader(ctx, nil), nil)
	if len(unfixed) != 0 || len(pins) != 1 || pins[0].Var != "test::Item::inner.fine" ||
		pins[0].Value.Kind != runtime.ValConst || pins[0].Value.Const.Real != 2.0 {
		t.Errorf("ChainPins fine = %+v, %+v, want inner.fine pinned to 2.0", pins, unfixed)
	}

	unbound, err := chainGuarded(t, guardedItemQueries(t, ctx, idx, "unbound"))
	if err != nil || len(unbound.question.Unreadable) != 0 {
		t.Fatalf("unbound = %v with Unreadable %+v, want a feature holding nothing left free", err, unbound.question.Unreadable)
	}
	free := varNamed(t, unbound.question, "test::Item::inner.free")
	var isFree bool
	for _, v := range unbound.question.Free() {
		isFree = isFree || v == free
	}
	if !isFree {
		t.Errorf("unbound Free() = %v, want inner.free", unbound.question.Free())
	}
}

// TestGuardedChainReadsDecideWhereTheEvaluatorDoes: solved, a nested declared
// value that does not evaluate decides the question exactly as a direct one —
// decided where no assignment reaches it, witnessed where a witness avoids it,
// undecided where the proof would need an assignment reaching it — and never
// as a value the solver made up for the feature.
func TestGuardedChainReadsDecideWhereTheEvaluatorDoes(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "nested_read.sysml", nestedReadSource)
	guarded := func(name string) guardedQueries {
		t.Helper()
		g, err := chainGuarded(t, guardedItemQueries(t, ctx, idx, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return g
	}
	chainAbsent := func(name string, q *Query, chain string) {
		t.Helper()
		for _, v := range q.Free() {
			if v.Name == chain {
				t.Errorf("%s: the unreadable %s is left free", name, chain)
			}
		}
	}

	// `true or inner.broken > 0.0` holds on every assignment, as the evaluator
	// says: the violation is unsat and no assignment reaches inner.broken.
	never := guarded("never")
	solved(t, solver, never.question, StatusSat)
	solved(t, solver, never.violation, StatusUnsat)
	solved(t, solver, never.question.Reached(), StatusUnsat)
	solved(t, solver, never.violation.Reached(), StatusUnsat)
	if result, err := ctx.CheckConstraintOn(never.sym, never.sym.OwnerScope, nil); err != nil || !result.Holds {
		t.Errorf("evaluate never = %+v, %v, want holds", result, err)
	}

	// `gate > 0.0 or inner.dependent > 0.0` is satisfied where gate > 0.0 without
	// reading inner.dependent, whose value the solver may not choose; its
	// violation has no model avoiding the read, and gate <= 0.0 reaches it.
	gated := guarded("gated")
	witness := modelValues(t, solved(t, solver, gated.question, StatusSat))
	chainAbsent("gated", gated.question, "test::Item::inner.dependent")
	if gate, ok := witness["test::Item::gate"]; !ok || strings.HasPrefix(gate, "-") || gate == "0.0" {
		t.Errorf("gated witness = %v, want gate > 0.0", witness)
	}
	solved(t, solver, gated.violation, StatusUnsat)
	solved(t, solver, gated.violation.Reached(), StatusSat)

	// `false or inner.dependent > 0.0` reaches inner.dependent on every
	// assignment: the guard admits none, and Reached says so.
	always := guarded("always")
	solved(t, solver, always.question, StatusUnsat)
	solved(t, solver, always.violation, StatusUnsat)
	solved(t, solver, always.question.Reached(), StatusSat)
	if _, err := ctx.CheckConstraintOn(always.sym, always.sym.OwnerScope, nil); !errors.Is(err, runtime.ErrNoValue) {
		t.Errorf("evaluate always = %v, want the evaluator failing on the read", err)
	}
}
