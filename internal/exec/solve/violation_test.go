package solve

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// violationQuery translates the named constraint as a violation query, failing
// the test when the translation refuses.
func violationQuery(t *testing.T, src, name string) *Query {
	t.Helper()
	ctx, idx := fixture(t, "<test>", src)
	sym := symbolNamed(t, idx, name)
	q, err := ConstraintViolation(ctx, sym, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("translate violation of %s: %v", name, err)
	}
	return q
}

// TestViolationQueryDeniesTheClaim: a violation query's models are the
// assignments violating the element's claim — the conjunction of its required
// conditions — asserted as one negation in role RoleViolated.
func TestViolationQueryDeniesTheClaim(t *testing.T) {
	q := violationQuery(t, constraintSource(`
		in level : Integer;
		assert constraint { level >= 0 }
		assert constraint { level <= 10 }
	`), "test::C")
	if !q.Violation {
		t.Fatal("the query does not record that it is a violation query")
	}
	last := q.Assertions[len(q.Assertions)-1]
	if last.From.Role != RoleViolated {
		t.Fatalf("last assertion is %s, want the violated conditions:\n%s", last.From.Role, Script(q))
	}
	want := "(not (and (>= |test::C::level| 0) (<= |test::C::level| 10)))"
	if got := writeTerm(last.Term); got != want {
		t.Errorf("violated assertion is %s, want %s", got, want)
	}
	if last.From.Condition != "not (level >= 0 and level <= 10)" {
		t.Errorf("condition is %q", last.From.Condition)
	}
}

// TestViolationQueryKeepsAssumptions: an assumption is a hypothesis of the
// claim, so a violation query keeps it asserted rather than negating it.
func TestViolationQueryKeepsAssumptions(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::Integer;
			part def Rig {
				attribute level : Integer;
				requirement within { assume constraint { level > 1 } require constraint { level < 5 } }
			}
		}
	`)
	sym := symbolNamed(t, idx, "test::Rig::within")
	q, err := RequirementViolation(ctx, sym, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(q.Assertions) != 2 {
		t.Fatalf("asserted %d terms, want the assumption and the violated claim:\n%s", len(q.Assertions), Script(q))
	}
	if q.Assertions[0].From.Role != RoleAssumed {
		t.Errorf("first assertion is %s, want the assumption kept", q.Assertions[0].From.Role)
	}
	if got := writeTerm(q.Assertions[1].Term); got != "(not (< |test::Rig::level| 5))" {
		t.Errorf("violated assertion is %s", got)
	}
}

// TestRequirementViolationAssumesHypotheses: a requirement's claim is its
// assumptions implying its required conditions, so the violation query keeps
// the assumptions and denies the requirements.
func TestRequirementViolationAssumesHypotheses(t *testing.T) {
	ctx, idx := fixtureFile(t, "touchdown.sysml")
	sym := symbolNamed(t, idx, "test::TouchdownRequirement")
	q, err := RequirementViolation(ctx, sym, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if !q.Violation || q.Kind != "requirement" {
		t.Fatalf("query is %s %+v", q.Kind, q)
	}
	var roles []Role
	for _, a := range q.Assertions {
		roles = append(roles, a.From.Role)
	}
	want := []Role{RoleAssumed, RoleViolated}
	if len(roles) != len(want) {
		t.Fatalf("roles are %v, want %v:\n%s", roles, want, Script(q))
	}
	for i, role := range want {
		if roles[i] != role {
			t.Errorf("assertion %d is %s, want %s", i, roles[i], role)
		}
	}
}

// TestNegatedViolationAffirmsTheConditions: the claim of `assert not` is that
// the required conditions do not all hold, so violating it is holding them all.
func TestNegatedViolationAffirmsTheConditions(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::Integer;
			part rig {
				attribute level : Integer;
				assert not constraint denied { level > 10 }
			}
		}
	`)
	sym := symbolNamed(t, idx, "test::rig::denied")
	q, err := ConstraintViolation(ctx, sym, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if !q.Violation || !q.Negated {
		t.Fatalf("query %+v does not record both flags", q)
	}
	last := q.Assertions[len(q.Assertions)-1]
	if last.From.Role != RoleViolated {
		t.Fatalf("last assertion is %s, want the violated conditions", last.From.Role)
	}
	// Violating `not (level > 10)` is `level > 10` itself: the double negation folds.
	if got := writeTerm(last.Term); got != "(> |test::rig::level| 10)" {
		t.Errorf("violated assertion is %s, want the required conditions", got)
	}
}

// TestSatisfactionViolationDeniesSatisfaction: an `assert satisfy` violation
// query's models are the assignments violating the requirement's claim.
func TestSatisfactionViolationDeniesSatisfaction(t *testing.T) {
	ctx, idx := fixtureFile(t, "satisfy_touchdown.sysml")
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("satisfy_touchdown.sysml"))
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}
	q, err := SatisfactionViolation(ctx, assertions[0], nil)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if !q.Violation || q.Kind != "satisfaction" {
		t.Fatalf("query is %s %+v", q.Kind, q)
	}
	last := q.Assertions[len(q.Assertions)-1]
	if last.From.Role != RoleViolated {
		t.Fatalf("last assertion is %s, want the violated conditions", last.From.Role)
	}
	want := "(not (<= |test::TouchdownRequirement::craft.verticalSpeed| |test::TouchdownRequirement::maxVerticalSpeed|))"
	if got := writeTerm(last.Term); got != want {
		t.Errorf("violated assertion is %s, want %s", got, want)
	}
}

// TestViolationComputedDivisorRefuses: a violation query hoists no definedness
// guard — the evaluator stops at the first failing required condition, so a
// guard hoisted from a later one could exclude real counterexamples and yield a
// false proof. A computed divisor therefore refuses even unbranched.
func TestViolationComputedDivisorRefuses(t *testing.T) {
	ctx, idx := fixture(t, "<test>", constraintSource(`
		in x : Real;
		in y : Real;
		assert constraint { x / y > 1.0 }
	`))
	sym := symbolNamed(t, idx, "test::C")
	q, err := ConstraintViolation(ctx, sym, sym.OwnerScope, nil)
	if q != nil || err == nil {
		t.Fatalf("violation translation succeeded, want a refusal:\n%s", Script(q))
	}
	var refused *NotTranslatableError
	if !errors.As(err, &refused) {
		t.Fatalf("translate: %v is not a NotTranslatableError", err)
	}
	if !strings.Contains(refused.Error(), "computed divisor") {
		t.Errorf("refusal is %q, want the computed-divisor reason", refused.Error())
	}
}

// TestViolationWithoutClaimRefuses: an element stating only assumptions claims
// nothing, so there is no claim to violate.
func TestViolationWithoutClaimRefuses(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::Integer;
			part def Rig {
				attribute level : Integer;
				assert constraint cn { assume constraint { level > 1 } }
			}
		}
	`)
	sym := symbolNamed(t, idx, "test::Rig::cn")
	if _, err := ConstraintViolation(ctx, sym, sym.OwnerScope, nil); !errors.Is(err, ErrNoConditions) {
		t.Fatalf("translate: %v, want ErrNoConditions", err)
	}
}

// TestViolationSolverVerdicts: over a solver, unsat proves the claim and sat is
// a replayed violation witness — the lemma the design checks by hand.
func TestViolationSolverVerdicts(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "<test>", `
		package P {
			private import ScalarValues::*;
			part hg { attribute efficiency : Real; attribute power : Real; }
			attribute d : Real;
			assert constraint lemma {
				(hg.efficiency >= 0.0 and hg.efficiency <= 1.0 and hg.power >= 0.0 and d >= 0.0)
				implies (hg.power * d * hg.efficiency) <= hg.power * d
			}
			assert constraint bad { hg.power * d <= hg.power }
		}
	`)
	q, err := ConstraintViolation(ctx, symbolNamed(t, idx, "P::lemma"), nil, nil)
	if err != nil {
		t.Fatalf("translate lemma: %v", err)
	}
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve lemma: %v", err)
	}
	if result.Status != StatusUnsat {
		t.Fatalf("lemma's violation is %s, want unsat — it holds for every assignment", result.Status)
	}
	if !result.RoundingProved {
		t.Error("lemma's unsat was not proved under the evaluator's float64 arithmetic")
	}

	bad, err := ConstraintViolation(ctx, symbolNamed(t, idx, "P::bad"), nil, nil)
	if err != nil {
		t.Fatalf("translate bad: %v", err)
	}
	result, err = solver.Solve(context.Background(), bad)
	if err != nil {
		t.Fatalf("solve bad: %v", err)
	}
	if result.Status != StatusSat {
		t.Fatalf("bad's violation is %s, want a witnessed violation", result.Status)
	}
	values := make(map[string]ModelValue, len(result.Model))
	for _, a := range result.Model {
		value, err := DecodeValue(a)
		if err != nil {
			t.Fatalf("decode %s: %v", a.Var.Name, err)
		}
		values[a.Var.Name] = value
	}
	if ok, why := bad.Confirm(values); !ok {
		t.Errorf("the evaluator does not confirm the violation witness: %s", why)
	}
}
