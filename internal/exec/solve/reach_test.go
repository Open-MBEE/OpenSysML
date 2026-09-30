package solve

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// shortCircuitSource declares a feature whose default does not evaluate and
// conditions reaching it under the evaluator's connectives in both operand
// orders, and a requirement reaching it only after an earlier required condition.
const shortCircuitSource = `package test {
	private import ScalarValues::*;
	part def Item {
		attribute bad : Real = 1.0 / 0.0;
		attribute free : Real;
		attribute other : Real;
		attribute input : Real;
		attribute dependent : Real = input + 1.0;
		assert constraint leftTrue { true or bad > 0.0 }
		assert constraint rightTrue { bad > 0.0 or true }
		assert constraint orFree { free > 0.0 or bad > 0.0 }
		assert constraint andFree { free > 0.0 and bad > 0.0 }
		assert constraint impliesFree { free > 0.0 implies bad > 0.0 }
		assert constraint xorFree { free > 0.0 xor bad > 0.0 }
		assert constraint bitAnd { free > 0.0 & bad > 0.0 }
		assert constraint bitOr { free > 0.0 | bad > 0.0 }
		assert constraint notOr { not (free > 0.0 or bad > 0.0) }
		assert constraint ifThen { if free > 0.0 ? bad > 0.0 else true }
		assert constraint ifElse { if free > 0.0 ? true else bad > 0.0 }
		assert constraint twice { (free > 0.0 or bad > 0.0) and (other > 0.0 or bad > 0.0) }
		assert constraint bothSides { (free > 0.0 or bad > 0.0) and bad > 0.0 }
		assert constraint nested { free > 0.0 or (other > 0.0 and bad > 0.0) }
		assert constraint orDependent { free > 0.0 or dependent > 0.0 }
	}
	part item : Item;
	requirement def Ordered {
		attribute bad : Real = 1.0 / 0.0;
		attribute free : Real;
		attribute other : Real;
		require constraint { free > 0.0 }
		assume constraint { other > 0.0 }
		require constraint { bad > 0.0 }
	}
}`

// varNamed finds the query's variable standing for a feature by its name.
func varNamed(t *testing.T, q *Query, name string) *Var {
	t.Helper()
	for _, v := range q.Vars {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("query has no variable %s among %v", name, q.Vars)
	return nil
}

// reachedText renders the condition under which a variable is read, "" where
// some reference reads it on every path.
func reachedText(v *Var) string {
	if v.Reached == nil {
		return ""
	}
	return writeTerm(v.Reached)
}

// TestReachedFollowsTheEvaluatorsOrder: a variable's Reached is the condition
// under which evalLogical reaches the reference reading it: the right operand of
// `and`, `&` and `implies` where the left holds, of `or` and `|` where it fails
// (shortCircuit decides `&` and `|` as it does `and` and `or`); both operands of
// `xor` always; the operand of `not` always; a branch of a conditional where its
// condition selects it; and a condition after another required one where that
// one holds — an assumption is reached the same way, but stops nothing itself.
func TestReachedFollowsTheEvaluatorsOrder(t *testing.T) {
	free := "(> |test::Item::free| 0.0)"
	cases := []struct {
		constraint, bad, other string
	}{
		{"leftTrue", "(not true)", ""},
		{"rightTrue", "", ""},
		{"orFree", "(not " + free + ")", ""},
		{"andFree", free, ""},
		{"impliesFree", free, ""},
		{"xorFree", "", ""},
		{"bitAnd", free, ""},
		{"bitOr", "(not " + free + ")", ""},
		{"notOr", "(not " + free + ")", ""},
		{"ifThen", free, ""},
		{"ifElse", "(not " + free + ")", ""},
		{"twice", "(or (not " + free + ") (and (or " + free + " (> |test::Item::bad| 0.0)) (not (> |test::Item::other| 0.0))))",
			"(or " + free + " (> |test::Item::bad| 0.0))"},
		{"bothSides", "(or (not " + free + ") (or " + free + " (> |test::Item::bad| 0.0)))", ""},
		{"nested", "(and (not " + free + ") (> |test::Item::other| 0.0))", "(not " + free + ")"},
	}
	for _, tc := range cases {
		q := constraintQuery(t, shortCircuitSource, "test::Item::"+tc.constraint)
		for _, v := range q.Vars {
			if v.Name == "test::Item::free" && v.Reached != nil {
				t.Errorf("%s: free reached under %s, want on every path", tc.constraint, reachedText(v))
			}
		}
		if got := reachedText(varNamed(t, q, "test::Item::bad")); got != tc.bad {
			t.Errorf("%s: bad reached under %q, want %q", tc.constraint, got, tc.bad)
		}
		if tc.other == "" {
			continue
		}
		if got := reachedText(varNamed(t, q, "test::Item::other")); got != tc.other {
			t.Errorf("%s: other reached under %q, want %q", tc.constraint, got, tc.other)
		}
	}

	ordered := requirementQuery(t, shortCircuitSource, "test::Ordered")
	if got := reachedText(varNamed(t, ordered, "test::Ordered::bad")); got != "(> |test::Ordered::free| 0.0)" {
		t.Errorf("Ordered: bad reached under %q, want after the first required condition only", got)
	}
	if got := reachedText(varNamed(t, ordered, "test::Ordered::other")); got != "(> |test::Ordered::free| 0.0)" {
		t.Errorf("Ordered: the assumption's other reached under %q, want after the first required condition", got)
	}
}

// guardedQueries translates a constraint of Item as a question about the item
// object would, in both modes, applying the unfixed values: the query the
// question asks and its violation.
type guardedQueries struct {
	pins      []Pin
	unfixed   []Unfixed
	sym       *symbols.Symbol
	ctx       *runtime.Context
	question  *Query
	violation *Query
}

func guardedItemQueries(t *testing.T, ctx *runtime.Context, idx *symbols.Index, name string) guardedQueries {
	t.Helper()
	owner := symbolNamed(t, idx, "test::Item")
	sym := symbolNamed(t, idx, "test::Item::"+name)
	item, err := ctx.Instantiate(symbolNamed(t, idx, "test::item"))
	if err != nil {
		t.Fatalf("instantiate item: %v", err)
	}
	pins, unfixed := FixedFor(ctx, Fixing{Element: sym, Owner: owner, Object: item, ObjectType: owner})
	g := guardedQueries{pins: pins, unfixed: unfixed, sym: sym, ctx: ctx}
	if g.question, err = ConstraintWith(ctx, sym, sym.OwnerScope, pins); err != nil {
		t.Fatalf("translate %s: %v", name, err)
	}
	if g.violation, err = ConstraintViolation(ctx, sym, sym.OwnerScope, pins); err != nil {
		t.Fatalf("translate the violation of %s: %v", name, err)
	}
	return g
}

// TestUnfixedReadGuardsAConditionalRead: a failed default the evaluator reaches
// only under some condition is not refused and not left free: the query asserts
// the condition false as a definedness guard ahead of the conditions, names the
// read in Unreadable, keeps the variable out of Free, and Reached asks whether
// any assignment reaches it. A read on every path still refuses, whatever the
// operand order says textually.
func TestUnfixedReadGuardsAConditionalRead(t *testing.T) {
	ctx, idx := fixture(t, "short_circuit.sysml", shortCircuitSource)

	left := guardedItemQueries(t, ctx, idx, "leftTrue")
	if err := UnfixedRead(ctx, left.question, left.unfixed); err != nil {
		t.Fatalf("UnfixedRead leftTrue = %v, want the unreached read guarded", err)
	}
	if len(left.question.Unreadable) != 1 || left.question.Unreadable[0].Name != "bad" ||
		!errors.Is(left.question.Unreadable[0].Err, runtime.ErrDivisionByZero) {
		t.Fatalf("Unreadable = %+v, want bad by division by zero", left.question.Unreadable)
	}
	bad := varNamed(t, left.question, "test::Item::bad")
	read := left.question.Unreadable[0]
	if read.Var != bad || read.Reached != bad.Reached || writeTerm(read.Guard) != "true" {
		t.Errorf("Unreadable = %+v, want bad guarded by (not (not true)) = true", read)
	}
	for _, v := range left.question.Free() {
		if v == bad {
			t.Errorf("Free() = %v includes the unreadable bad", left.question.Free())
		}
	}
	var guard *Assertion
	for i := range left.question.Assertions {
		a := &left.question.Assertions[i]
		if a.Term == read.Guard {
			guard = a
		}
		if guard == nil && a.From.Role == RoleRequired {
			t.Errorf("required condition %q precedes the guard", a.From.Condition)
		}
	}
	if guard == nil {
		t.Fatalf("no guard asserted; script:\n%s", Script(left.question))
	}
	if guard.From.Role != RoleDefined || !strings.Contains(guard.From.Condition, "bad is not read") ||
		!strings.Contains(guard.From.Condition, "division by zero") {
		t.Errorf("guard = %+v, want role %s naming bad and the read failure", guard.From, RoleDefined)
	}
	if err := UnfixedRead(ctx, left.question, left.unfixed); err != nil || len(left.question.Unreadable) != 1 {
		t.Errorf("a second UnfixedRead = %v with %d unreadable, want the first guard kept", err, len(left.question.Unreadable))
	}

	reached := left.question.Reached()
	if reached == nil || reached.Kind != left.question.Kind || reached.Element != left.question.Element {
		t.Fatalf("Reached() = %+v, want a query about the same element", reached)
	}
	var required int
	for _, a := range reached.Assertions {
		switch {
		case a.Term == read.Guard:
			t.Errorf("Reached() keeps the guard %q", a.From.Condition)
		case a.From.Role == RoleRequired:
			required++
			if a.Term != read.Reached || !strings.Contains(a.From.Condition, "bad is read") {
				t.Errorf("Reached() requires %q (%s), want the read condition", a.From.Condition, writeTerm(a.Term))
			}
		case !contextRole(a.From.Role) && a.From.Role != RoleAssumed:
			t.Errorf("Reached() asserts %s %q, want only the context", a.From.Role, a.From.Condition)
		}
	}
	if required != 1 {
		t.Errorf("Reached() requires %d conditions, want the reads only; script:\n%s", required, Script(reached))
	}
	if err := left.question.ReachedError(); err == nil || !errors.Is(err, runtime.ErrDivisionByZero) ||
		!strings.HasPrefix(err.Error(), "constraint leftTrue reads bad under some assignment of its free features, whose value could not be read: ") ||
		!strings.HasSuffix(err.Error(), "division by zero") {
		t.Errorf("ReachedError() = %v", err)
	}

	right := guardedItemQueries(t, ctx, idx, "rightTrue")
	err := UnfixedRead(ctx, right.question, right.unfixed)
	if err == nil || !errors.Is(err, runtime.ErrDivisionByZero) ||
		!strings.HasPrefix(err.Error(), "constraint rightTrue reads bad, whose value could not be read: ") ||
		!strings.HasSuffix(err.Error(), "division by zero") {
		t.Errorf("UnfixedRead rightTrue = %v, want the read on every path refused", err)
	}
	if len(right.question.Unreadable) != 0 || right.question.Reached() != nil || right.question.ReachedError() != nil {
		t.Errorf("a refused read left Unreadable = %+v", right.question.Unreadable)
	}
	xor := guardedItemQueries(t, ctx, idx, "xorFree")
	if err := UnfixedRead(ctx, xor.question, xor.unfixed); err == nil || !strings.Contains(err.Error(), "reads bad, whose value") {
		t.Errorf("UnfixedRead xorFree = %v, want the read of both operands refused", err)
	}

	dependent := guardedItemQueries(t, ctx, idx, "orDependent")
	if err := UnfixedRead(ctx, dependent.question, dependent.unfixed); err != nil {
		t.Fatalf("UnfixedRead orDependent = %v, want the unreached missing dependency guarded", err)
	}
	if len(dependent.question.Unreadable) != 1 || !errors.Is(dependent.question.Unreadable[0].Err, runtime.ErrNoValue) {
		t.Errorf("Unreadable = %+v, want dependent by a missing value", dependent.question.Unreadable)
	}
	if err := dependent.question.ReachedError(); !errors.Is(err, runtime.ErrNoValue) || !strings.Contains(err.Error(), "dependent") {
		t.Errorf("ReachedError() = %v, want ErrNoValue naming dependent", err)
	}
}

// TestGuardedReadsDecideWhereTheEvaluatorDoes: solved, the guarded queries
// answer for the assignments that never reach the failed default, and Reached
// says whether an assignment does: unsat where the evaluator decides on every
// assignment, sat where some assignment leaves it without a verdict.
func TestGuardedReadsDecideWhereTheEvaluatorDoes(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "short_circuit.sysml", shortCircuitSource)
	guarded := func(name string) guardedQueries {
		t.Helper()
		g := guardedItemQueries(t, ctx, idx, name)
		if err := UnfixedRead(ctx, g.question, g.unfixed); err != nil {
			t.Fatalf("UnfixedRead %s: %v", name, err)
		}
		if err := UnfixedRead(ctx, g.violation, g.unfixed); err != nil {
			t.Fatalf("UnfixedRead the violation of %s: %v", name, err)
		}
		return g
	}
	badAbsent := func(name string, q *Query) {
		t.Helper()
		for _, v := range q.Free() {
			if v.Name == "test::Item::bad" {
				t.Errorf("%s: the unreadable bad is left free", name)
			}
		}
	}

	// `true or bad > 0.0` holds on every assignment: the violation is unsat and
	// no assignment reaches bad.
	left := guarded("leftTrue")
	solved(t, solver, left.question, StatusSat)
	solved(t, solver, left.violation, StatusUnsat)
	solved(t, solver, left.question.Reached(), StatusUnsat)
	solved(t, solver, left.violation.Reached(), StatusUnsat)

	// `free > 0.0 or bad > 0.0` is satisfied where free > 0.0, and its violation
	// has no model avoiding bad — but an assignment with free <= 0.0 reaches it,
	// so the proof stays undecided.
	or := guarded("orFree")
	witness := modelValues(t, solved(t, solver, or.question, StatusSat))
	badAbsent("orFree", or.question)
	if free, ok := witness["test::Item::free"]; !ok || strings.HasPrefix(free, "-") || free == "0.0" {
		t.Errorf("orFree witness = %v, want free > 0.0", witness)
	}
	solved(t, solver, or.violation, StatusUnsat)
	solved(t, solver, or.violation.Reached(), StatusSat)

	// `free > 0.0 and bad > 0.0` is violated where free <= 0.0, a counterexample
	// the evaluator confirms; no assignment satisfies it without reaching bad.
	and := guarded("andFree")
	counter := modelValues(t, solved(t, solver, and.violation, StatusSat))
	badAbsent("andFree", and.violation)
	if free, ok := counter["test::Item::free"]; !ok || !(strings.HasPrefix(free, "-") || free == "0.0") {
		t.Errorf("andFree counterexample = %v, want free <= 0.0", counter)
	}
	solved(t, solver, and.question, StatusUnsat)
	solved(t, solver, and.question.Reached(), StatusSat)

	// `free > 0.0 implies bad > 0.0` reaches bad where free > 0.0 and holds
	// elsewhere: no counterexample avoids bad, and some assignment reaches it.
	implies := guarded("impliesFree")
	solved(t, solver, implies.violation, StatusUnsat)
	solved(t, solver, implies.violation.Reached(), StatusSat)
	solved(t, solver, implies.question, StatusSat)

	// A conditional reaches bad only in the branch its condition selects.
	ifElse := guarded("ifElse")
	solved(t, solver, ifElse.violation, StatusUnsat)
	solved(t, solver, ifElse.violation.Reached(), StatusSat)
	ifThen := guarded("ifThen")
	solved(t, solver, ifThen.violation, StatusUnsat)
	solved(t, solver, ifThen.violation.Reached(), StatusSat)

	// `(free > 0.0 or bad > 0.0) and bad > 0.0` reaches bad on every assignment,
	// one operand or the other: nothing is decided.
	both := guarded("bothSides")
	solved(t, solver, both.question, StatusUnsat)
	solved(t, solver, both.violation, StatusUnsat)
	solved(t, solver, both.question.Reached(), StatusSat)
}

// TestRequirementReachesALaterConditionAfterTheEarlierOnes: in a requirement,
// a failed default read by a later required condition is reached only where the
// earlier required conditions hold, so its violation has a real counterexample
// where the first fails; assumptions gate nothing.
func TestRequirementReachesALaterConditionAfterTheEarlierOnes(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "short_circuit.sysml", shortCircuitSource)
	sym := symbolNamed(t, idx, "test::Ordered")
	pins, unfixed := FixedFor(ctx, Fixing{Element: sym, Owner: sym})
	violation, err := RequirementViolation(ctx, sym, sym.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate the violation of Ordered: %v", err)
	}
	if err := UnfixedRead(ctx, violation, unfixed); err != nil {
		t.Fatalf("UnfixedRead Ordered = %v, want the later read guarded", err)
	}
	if len(violation.Unreadable) != 1 || violation.Unreadable[0].Name != "bad" {
		t.Fatalf("Unreadable = %+v, want bad", violation.Unreadable)
	}
	counter := modelValues(t, solved(t, solver, violation, StatusSat))
	if free, ok := counter["test::Ordered::free"]; !ok || !(strings.HasPrefix(free, "-") || free == "0.0") {
		t.Errorf("counterexample = %v, want free <= 0.0 failing the first condition", counter)
	}
	if other, ok := counter["test::Ordered::other"]; !ok || strings.HasPrefix(other, "-") || other == "0.0" {
		t.Errorf("counterexample = %v, want the assumption other > 0.0 kept", counter)
	}
	solved(t, solver, violation.Reached(), StatusSat)
}

// laterGuardSource declares requirements whose failed default is reached under
// a condition, and a computed divisor the evaluator checks after that read
// (Pinned, Selected) or before it (Preceded).
const laterGuardSource = `package test {
	private import ScalarValues::*;
	requirement def Pinned {
		attribute bad : Real = 1.0 / 0.0;
		attribute free : Real;
		attribute divisor : Real = 0.0;
		attribute a : Real;
		require constraint { free > 0.0 or bad > 0.0 }
		require constraint { a / divisor > 0.0 }
	}
	requirement def Selected {
		attribute bad : Real = 1.0 / 0.0;
		attribute free : Real;
		require constraint { free > 0.0 or bad > 0.0 }
		require constraint { 1.0 / (if free > 0.0 ? 1.0 else 0.0) > 2.0 }
	}
	requirement def Preceded {
		attribute bad : Real = 1.0 / 0.0;
		attribute free : Real;
		require constraint { 1.0 / (if free > 0.0 ? 1.0 else 0.0) > 0.5 }
		require constraint { free > 0.0 or bad > 0.0 }
	}
}`

// TestReachedKeepsOnlyTheGuardsCheckedBeforeTheRead: a definedness guard
// hoisted from an operation the evaluator performs only after reaching a failed
// default — a later required condition's divisor — does not constrain Reached:
// an assignment failing at the read never performs the operation, so the guard
// would hide the assignment and certify a false proof where the question is
// unsat for its own reasons. A guard the evaluator checks before the read does
// constrain it: an assignment failing that check never reaches the read.
func TestReachedKeepsOnlyTheGuardsCheckedBeforeTheRead(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "later_guard.sysml", laterGuardSource)
	guarded := func(name string) *Query {
		t.Helper()
		sym := symbolNamed(t, idx, "test::"+name)
		pins, unfixed := FixedFor(ctx, Fixing{Element: sym, Owner: sym})
		q, err := RequirementWith(ctx, sym, sym.OwnerScope, pins)
		if err != nil {
			t.Fatalf("translate %s: %v", name, err)
		}
		if err := UnfixedRead(ctx, q, unfixed); err != nil {
			t.Fatalf("UnfixedRead %s = %v, want the read under free <= 0.0 guarded", name, err)
		}
		if len(q.Unreadable) != 1 || q.Unreadable[0].Name != "bad" {
			t.Fatalf("%s Unreadable = %+v, want bad", name, q.Unreadable)
		}
		return q
	}
	definedness := func(q *Query) []string {
		var out []string
		for _, a := range q.Assertions {
			if a.From.Role == RoleDefined {
				out = append(out, writeTerm(a.Term))
			}
		}
		return out
	}

	// The divisor pinned to 0.0 makes the question unsat through its hoisted
	// guard; free <= 0.0 still reaches bad first, so no proof is certified.
	pinned := guarded("Pinned")
	if got := definedness(pinned); len(got) != 2 || got[0] != "(distinct |test::Pinned::divisor| 0.0)" {
		t.Errorf("Pinned definedness guards = %q, want the divisor's and the read's", got)
	}
	if got := pinned.Unreadable[0].Preceding; len(got) != 0 {
		t.Errorf("Pinned: guards preceding the read = %v, want none: the division comes after", got)
	}
	solved(t, solver, pinned, StatusUnsat)
	reached := pinned.Reached()
	if got := definedness(reached); len(got) != 0 {
		t.Errorf("Pinned Reached() asserts definedness guards %q, want none", got)
	}
	values := modelValues(t, solved(t, solver, reached, StatusSat))
	if free, ok := values["test::Pinned::free"]; !ok || !(strings.HasPrefix(free, "-") || free == "0.0") {
		t.Errorf("Pinned Reached() model = %v, want free <= 0.0 reaching bad", values)
	}

	// A divisor selected by free itself: its guard is free > 0.0, which the
	// read's condition contradicts, yet the evaluator checks it only after.
	selected := guarded("Selected")
	solved(t, solver, selected, StatusUnsat)
	solved(t, solver, selected.Reached(), StatusSat)

	// Checked before the read, the same guard says which assignments reach it:
	// none with free <= 0.0, as the evaluator fails dividing there first.
	preceded := guarded("Preceded")
	if got := preceded.Unreadable[0].Preceding; len(got) != 1 ||
		writeTerm(got[0]) != "(distinct (ite (> |test::Preceded::free| 0.0) 1.0 0.0) 0.0)" {
		t.Errorf("Preceded: guards preceding the read = %v, want the divisor's", got)
	}
	values = modelValues(t, solved(t, solver, preceded, StatusSat))
	if free, ok := values["test::Preceded::free"]; !ok || strings.HasPrefix(free, "-") || free == "0.0" {
		t.Errorf("Preceded witness = %v, want free > 0.0", values)
	}
	solved(t, solver, preceded.Reached(), StatusUnsat)
}
