package solve

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// synthesised is the witness a solver answered with, by qualified feature name.
func synthesised(result *Result) map[string]string {
	out := map[string]string{}
	for _, a := range result.Model {
		out[a.Var.Name] = a.Value
	}
	return out
}

// TestSynthesisFillsWhatIsNotFixed: with the declared width fixed, the solver
// answers with a height the constraint permits — a property of the witness, since
// any height under the bound is as good an answer.
func TestSynthesisFillsWhatIsNotFixed(t *testing.T) {
	solver := requireSolver(t)
	_, _, q := fixedQuery(t, "test::Panel::fits")
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusSat {
		t.Fatalf("status %s, want values satisfying the constraint", result.Status)
	}
	values := synthesised(result)
	if values["test::Panel::width"] != "4" {
		t.Errorf("the width is %q, want the declared 4", values["test::Panel::width"])
	}
	height, ok := values["test::Panel::height"]
	if !ok {
		t.Fatalf("no height was synthesised: %+v", values)
	}
	if n := atoiOrFail(t, height); 4+n > 10 {
		t.Errorf("the height %s does not satisfy width + height <= 10", height)
	}
}

// TestSynthesisIsUnsatWhenTheFixedValuesForbidIt: with the declared finish and
// width fixed, no values remain, which is the fixed values conflicting rather
// than the constraint being unsatisfiable on its own.
func TestSynthesisIsUnsatWhenTheFixedValuesForbidIt(t *testing.T) {
	solver := requireSolver(t)
	_, _, q := fixedQuery(t, "test::Panel::polishedIsWide")
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusUnsat {
		t.Fatalf("status %s, want the fixed values to conflict", result.Status)
	}

	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	sym := symbolNamed(t, idx, "test::Panel::polishedIsWide")
	free, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate without fixed values: %v", err)
	}
	plain, err := solver.Solve(context.Background(), free)
	if err != nil {
		t.Fatalf("solve without fixed values: %v", err)
	}
	if plain.Status != StatusSat {
		t.Fatalf("status %s, want the constraint itself to be satisfiable", plain.Status)
	}
}

// TestFixedValuesInAConflictAreNamed: the core of a conflict names the fixed
// values taking part, which is what tells a reader which of them to change.
func TestFixedValuesInAConflictAreNamed(t *testing.T) {
	solver := requireSolver(t)
	_, _, q := fixedQuery(t, "test::Panel::polishedIsWide")
	result, err := solver.Explain(context.Background(), q)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if result.Status != StatusUnsat {
		t.Fatalf("status %s, want a conflict to explain", result.Status)
	}
	if result.Core == nil {
		t.Skip("this solver produced no core")
	}
	named := map[string]bool{}
	for _, member := range result.Core.Members {
		if member.From.Role == RolePinned {
			named[member.From.Element] = true
		}
	}
	for _, want := range []string{"test::Panel::finish", "test::Panel::width"} {
		if !named[want] {
			t.Errorf("the core does not name the value fixed for %s: %+v", want, result.Core.Members)
		}
	}
}

// TestSynthesisRespectsAssumedConditions: an assumption holds of the fixed
// values, so what remains is the required condition, which they break.
func TestSynthesisRespectsAssumedConditions(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixtureFile(t, "rig_pins.sysml")
	landing := symbolNamed(t, idx, "test::Landing")
	pins, unfixed := Fixed(ctx, landing, nil)
	if len(unfixed) != 0 {
		t.Fatalf("declared values could not be read: %+v", unfixed)
	}
	q, err := RequirementWith(ctx, landing, landing.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate with fixed values: %v", err)
	}
	roles := map[Role]int{}
	for _, a := range q.Assertions {
		roles[a.From.Role]++
	}
	if roles[RoleAssumed] == 0 || roles[RoleRequired] == 0 || roles[RolePinned] != 2 {
		t.Fatalf("assertions by role %+v, want the assumption, the requirement and both fixed values", roles)
	}
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusUnsat {
		t.Fatalf("status %s, want 3.0 [m/s] against a limit of 5.4 [km/h] to conflict", result.Status)
	}
}

// TestSynthesisUnderANaturalDomainAndADivisorGuard: a Natural stays at or above
// zero and a divisor stays away from it, so the value synthesised for the divisor
// is one the evaluator would divide by.
func TestSynthesisUnderANaturalDomainAndADivisorGuard(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixtureFile(t, "rig_pins.sysml")
	rig := symbolNamed(t, idx, "test::Rig")
	pins, unfixed := Fixed(ctx, rig, nil)
	if len(unfixed) != 0 {
		t.Fatalf("declared values could not be read: %+v", unfixed)
	}
	sym := symbolNamed(t, idx, "test::Rig::sharing")
	q, err := ConstraintWith(ctx, sym, sym.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate with fixed values: %v", err)
	}
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusSat {
		t.Fatalf("status %s, want a number of shares satisfying the constraint", result.Status)
	}
	values := synthesised(result)
	shares := atoiOrFail(t, values["test::Rig::shares"])
	if shares < 1 {
		t.Fatalf("the shares are %d, which no division by them is guarded for", shares)
	}
	if 7/shares < 2 {
		t.Errorf("7 / %d is under 2, so the synthesised shares do not satisfy the constraint", shares)
	}
}

// TestSynthesisForADeniedElement: a usage denying a constraint permits only
// values breaking it, so a value satisfying it cannot be fixed and hold.
func TestSynthesisForADeniedElement(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixtureFile(t, "rig_pins.sysml")
	sym := symbolNamed(t, idx, "test::frame::notWide")
	width := symbolNamed(t, idx, "test::Wide::width")
	wide := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 25}}
	q, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: width, Name: "width", Value: wide, Source: PinChosen,
	}})
	if err != nil {
		t.Fatalf("translate with a chosen width: %v", err)
	}
	if !q.Negated {
		t.Fatal("the usage denies the constraint, which the query does not record")
	}
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusUnsat {
		t.Fatalf("status %s, want a width of 25 to conflict with denying width >= 20", result.Status)
	}

	narrow := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 19}}
	permitted, err := ConstraintWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: width, Name: "width", Value: narrow, Source: PinChosen,
	}})
	if err != nil {
		t.Fatalf("translate with a narrower width: %v", err)
	}
	ok, err := solver.Solve(context.Background(), permitted)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if ok.Status != StatusSat {
		t.Fatalf("status %s, want a width of 19 to satisfy the denial", ok.Status)
	}
}

// TestWitnessIsReportedInOpenSysMLTerms: the values reported name the features
// and the units the notation writes, not the solver's own text.
func TestWitnessIsReportedInOpenSysMLTerms(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixtureFile(t, "rig_pins.sysml")
	landing := symbolNamed(t, idx, "test::Landing")
	q, err := Requirement(ctx, landing, landing.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusSat {
		t.Fatalf("status %s, want the requirement to be satisfiable", result.Status)
	}
	for _, a := range result.Model {
		if !strings.HasPrefix(a.Var.Name, "test::Landing::") {
			t.Errorf("the witness names %s, which is not a feature of the requirement", a.Var.Name)
		}
		if !strings.Contains(a.Value, "[") {
			t.Errorf("%s = %q states no unit, though its values are measured", a.Var.Name, a.Value)
		}
	}
}

// atoiOrFail reads an integer the solver answered with.
func atoiOrFail(t *testing.T, text string) int {
	t.Helper()
	n, err := strconv.Atoi(text)
	if err != nil {
		t.Fatalf("%q is no integer: %v", text, err)
	}
	return n
}
