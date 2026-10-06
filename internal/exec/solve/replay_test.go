package solve

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
)

// The half-ulp quotient (2^53 + 1) / 2 is not representable in float64, and
// both the evaluator and the encoding hold it as the exact Rational. The replay
// confirms the witness under the evaluator's arithmetic.
func TestSolvedHalfUlpQuotientAgreesWithEvaluator(t *testing.T) {
	solver := requireSolver(t)
	wantQ, wantR := evaluatedDivision(t, 9007199254740993, 2)
	q := constraintQuery(t, `
		package test {
			private import ScalarValues::Integer;
			private import ScalarValues::Rational;
			constraint def C {
				in a : Integer; in q : Rational; in r : Integer;
				assert constraint { a == 9007199254740993 }
				assert constraint { q == a / 2 }
				assert constraint { r == a % 2 }
			}
		}`, "test::C")
	values := modelValues(t, solved(t, solver, q, StatusSat))
	if !sameReal(t, values["test::C::q"], wantQ) || values["test::C::r"] != wantR {
		t.Errorf("solved 9007199254740993 / 2 = %s remainder %s, evaluator says %s remainder %s",
			values["test::C::q"], values["test::C::r"], wantQ, wantR)
	}

	// Only the exact ratio satisfies the evaluator and the encoding alike, so
	// the query is not marked rounded: its unsat is the evaluator's verdict.
	for _, tc := range []struct {
		quotient string
		want     Status
	}{
		{"4503599627370496.5", StatusSat},
		{"4503599627370496.0", StatusUnsat},
	} {
		cond := constraintQuery(t, fmt.Sprintf(`
			package test {
				private import ScalarValues::Integer;
				constraint def C {
					in a : Integer;
					assert constraint { a == 9007199254740993 }
					assert constraint { a / 2 == %s }
				}
			}`, tc.quotient), "test::C")
		if cond.Rounded() {
			t.Errorf("an exact Integer quotient query is marked rounded")
		}
		solved(t, solver, cond, tc.want)
	}
}

// A witness satisfying the exact encoding but not the evaluator's float64
// arithmetic is not reported sat: the replay leaves the question undecided.
func TestSolvedWitnessRejectedByEvaluatorIsUndecided(t *testing.T) {
	solver := requireSolver(t)
	cases := map[string]string{
		// Exactly x = 1/10 solves this, but float64 0.1 * 3.0 is not float64 0.3.
		"product": `in x : Real;
			assert constraint { x * 3.0 == 0.3 }`,
		// Exactly 1/10 + 2/10 == 3/10, but Reals hold binary64 values, and no
		// float64 is one tenth.
		"sum": `in x : Real; in y : Real;
			assert constraint { x == 0.1 }
			assert constraint { y == 0.2 }
			assert constraint { x + y == 0.3 }`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			q := constraintQuery(t, fmt.Sprintf(`
				package test {
					private import ScalarValues::Real;
					constraint def C { %s }
				}`, body), "test::C")
			result := solved(t, solver, q, StatusUnknown)
			if !strings.Contains(result.Reason, "evaluator") {
				t.Errorf("reason %q does not say the evaluator rejected the witness", result.Reason)
			}
		})
	}
}

// A witness the evaluator confirms keeps its sat verdict: the replay is a
// filter on false claims, not on true ones.
func TestSolvedWitnessConfirmedByEvaluatorStaysSat(t *testing.T) {
	solver := requireSolver(t)
	q := constraintQuery(t, `
		package test {
			private import ScalarValues::Real;
			constraint def C {
				in x : Real; in y : Real;
				assert constraint { x == 0.5 }
				assert constraint { y == 0.25 }
				assert constraint { x + y == 0.75 }
			}
		}`, "test::C")
	solved(t, solver, q, StatusSat)
}

// Rational arithmetic is exact in the evaluator (KerML 9.3.2.2.8), so the exact
// encoding's witness is confirmed and the query makes no rounded claim.
func TestSolvedExactRationalWitnessStaysSat(t *testing.T) {
	solver := requireSolver(t)
	cases := map[string]string{
		"sum": `in x : Rational; in y : Rational;
			assert constraint { x == 0.1 }
			assert constraint { y == 0.2 }
			assert constraint { x + y == 0.3 }`,
		"product": `in x : Rational;
			assert constraint { x * 3.0 == 0.3 }`,
		// No variables at all: the empty witness is still replayed.
		"constants": `assert constraint { 0.1 + 0.2 == 0.3 }`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			q := constraintQuery(t, fmt.Sprintf(`
				package test {
					private import ScalarValues::Rational;
					constraint def C { %s }
				}`, body), "test::C")
			if q.Rounded() {
				t.Error("an exact Rational query is marked rounded")
			}
			solved(t, solver, q, StatusSat)
		})
	}
}

// Rounded marks exactly the queries whose conditions the evaluator computes in
// float64; a query over integers alone makes no rounded claim.
func TestRoundedMarksFloatComputingQueries(t *testing.T) {
	rounded := constraintQuery(t, `
		package test {
			private import ScalarValues::Real;
			constraint def C {
				in x : Real;
				assert constraint { x + 0.5 == 1.5 }
			}
		}`, "test::C")
	if !rounded.Rounded() {
		t.Error("a real-sum query is not marked rounded")
	}
	exact := constraintQuery(t, `
		package test {
			private import ScalarValues::Integer;
			constraint def C {
				in i : Integer;
				assert constraint { i + 1 > 3 }
			}
		}`, "test::C")
	if exact.Rounded() {
		t.Error("an integer-only query is marked rounded")
	}
	x := &Var{Name: "x", Sort: Real, Binary64: true}
	objective := &Query{Objectives: []Objective{{
		Term: Binary(OpAdd, Real, VarTerm(x), RealTerm(big.NewRat(1, 5))),
	}}}
	if !objective.Rounded() {
		t.Error("a query optimizing a real sum is not marked rounded")
	}
	rational := &Query{Objectives: []Objective{{
		Term: Binary(OpAdd, Real, RealTerm(big.NewRat(1, 10)), RealTerm(big.NewRat(1, 5))),
	}}}
	if rational.Rounded() {
		t.Error("a query optimizing an exact Rational sum is marked rounded")
	}
}
