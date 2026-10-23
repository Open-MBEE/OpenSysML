package solve

import (
	"errors"
	"strings"
	"testing"
)

// satisfaction returns the single satisfaction assertion of a fixture.
func satisfaction(t *testing.T, name string) *Query {
	t.Helper()
	ctx, idx := fixtureFile(t, name)
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot(name))
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}
	q, err := Satisfaction(ctx, assertions[0])
	if err != nil {
		t.Fatalf("translate satisfaction: %v", err)
	}
	return q
}

// TestSatisfaction translates the conditions an `assert satisfy` states: those of
// the requirement it references, read through the requirement's own parameters.
// The values the subject holds are not asserted — the query asks what the
// conditions permit, which is the question a solver answers.
func TestSatisfaction(t *testing.T) {
	q := satisfaction(t, "satisfy_touchdown.sysml")
	if q.Kind != "satisfaction" || !strings.HasPrefix(q.Element, "satisfy touchdown") {
		t.Errorf("query is about %s %q", q.Kind, q.Element)
	}
	if len(q.Assertions) != 1 {
		t.Fatalf("asserted %d terms:\n%s", len(q.Assertions), Script(q))
	}
	want := "(<= |test::TouchdownRequirement::craft.verticalSpeed| |test::TouchdownRequirement::maxVerticalSpeed|)"
	if got := writeTerm(q.Assertions[0].Term); got != want {
		t.Errorf("assertion is %s, want %s", got, want)
	}
	compareGolden(t, "satisfy_touchdown.smt2", Script(q))
}

// TestSatisfactionWithoutConditionsIsVacuous: a requirement stating no condition
// is true of every assignment, so the satisfaction's query asserts nothing and
// is satisfiable, while its denial has nothing to deny.
func TestSatisfactionWithoutConditionsIsVacuous(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			requirement def Documented { doc /* prose only */ }
			requirement documented : Documented;
			part lander;
			part context {
				assert satisfy documented by lander;
				assert not satisfy documented by lander;
			}
		}
	`)
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 2 {
		t.Fatalf("found %d satisfaction assertions, want 2", len(assertions))
	}
	q, err := Satisfaction(ctx, assertions[0])
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if q.Kind != "satisfaction" || len(q.Assertions) != 0 || len(q.Vars) != 0 {
		t.Fatalf("a vacuous satisfaction asserts something:\n%s", Script(q))
	}
	if _, err := Satisfaction(ctx, assertions[1]); !errors.Is(err, ErrNoConditions) {
		t.Fatalf("translate the denial: %v, want ErrNoConditions", err)
	}
	// The claim holds vacuously, so nothing violates it: the violation query is false.
	v, err := SatisfactionViolation(ctx, assertions[0], nil)
	if err != nil {
		t.Fatalf("translate the violation: %v", err)
	}
	if len(v.Assertions) != 1 || writeTerm(v.Assertions[0].Term) != "(not true)" {
		t.Fatalf("the violation of a vacuous satisfaction is not false:\n%s", Script(v))
	}
}

// TestNegatedSatisfactionDenies: `assert not satisfy` holds when the requirement
// is not satisfied, so the query denies the conjunction of its conditions.
func TestNegatedSatisfactionDenies(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::Real;
			part def Lander { attribute verticalSpeed : Real; }
			requirement def R {
				subject craft : Lander;
				require constraint { craft.verticalSpeed <= 1.5 }
			}
			requirement touchdown : R;
			part lander : Lander;
			part context {
				assert not satisfy touchdown by lander;
			}
		}
	`)
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	q, err := Satisfaction(ctx, assertions[0])
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if !q.Negated || q.Assertions[0].From.Role != RoleDenied {
		t.Fatalf("the query does not deny the requirement's conditions:\n%s", Script(q))
	}
	want := "(not (<= |test::R::craft.verticalSpeed| 1.5))"
	if got := writeTerm(q.Assertions[0].Term); got != want {
		t.Errorf("denial is %s, want %s", got, want)
	}
}
