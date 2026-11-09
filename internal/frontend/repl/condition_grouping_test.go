package repl

import (
	"strings"
	"testing"
)

// The reporter's model: a conjunction whose right operand is a disjunction, which
// the parser folds to its tree and the verdict must print with the grouping kept.
const parensFixture = `
package Parens {
    private import ScalarValues::*;
    part q {
        attribute a : Boolean = true;
        attribute b : Boolean = false;
        attribute c : Boolean = false;
        assert constraint d { a and (b or c) }
        assert not constraint e { b or c }
        assert constraint f { (a and b) or c }
    }
}
`

// TestValidatePrintsConditionGrouping checks %validate and %constraint print a
// condition as the tree they evaluated: `a and (b or c)`, not `a and b or c`.
func TestValidatePrintsConditionGrouping(t *testing.T) {
	s := loadSource(t, parensFixture)
	wants(t, run(t, s, "%instantiate Parens::q"), "✓ Created instance of Parens::q")

	out := run(t, s, "%validate q")
	wantsInOrder(t, out,
		"✗ assert constraint d fails (on Parens::q ID: 1)",
		"Assertion evaluated to false: a and (b or c)",
		"✓ assert not constraint e holds (on Parens::q ID: 1)",
		"✗ assert constraint f fails (on Parens::q ID: 1)",
		"Assertion evaluated to false: a and b or c",
	)
	if strings.Contains(out, "(a and b) or c") {
		t.Errorf("%%validate reinstated parentheses that change nothing:\n%s", out)
	}

	wantVerdict(t, s.CheckConstraint("Parens::q::d"), VerdictFails,
		"✗ Constraint Parens::q::d failed",
		"Assertion evaluated to false: a and (b or c)")
	wantVerdict(t, s.CheckConstraint("Parens::q::e"), VerdictHolds,
		"✓ Constraint Parens::q::e passed")
}

// TestNegatedDisjunctionIsGrouped checks a violated `not` over a disjunction is
// reported as `not (b or c)`, which reads back as the negation of the whole.
func TestNegatedDisjunctionIsGrouped(t *testing.T) {
	s := loadSource(t, strings.Replace(parensFixture, "b : Boolean = false", "b : Boolean = true", 1))
	run(t, s, "%instantiate Parens::q")
	wantsInOrder(t, run(t, s, "%validate q"),
		"✗ assert not constraint e fails (on Parens::q ID: 1)",
		"Assertion evaluated to false: not (b or c)",
	)
}

// TestExplainPrintsConditionGrouping checks a conflict line keeps the grouping
// of a mixed and/or condition rather than flattening it to `x > 1.0 and x < 0.0 or …`.
func TestExplainPrintsConditionGrouping(t *testing.T) {
	requireSolver(t)
	s := loadSource(t, `
package Mixed {
    private import ScalarValues::Real;
    constraint def Band {
        in x : Real;
        assert constraint { x > 1.0 and (x < 0.0 or x > 5.0) and x < 3.0 }
    }
}
`)
	got := solverAgnostic(run(t, s, "%explain Mixed::Band"))
	wants(t, got, "required condition: `x > 1.0 and (x < 0.0 or x > 5.0) and x < 3.0`")
	if strings.Contains(got, "x > 1.0 and x < 0.0 or") {
		t.Errorf("%%explain flattened the condition:\n%s", got)
	}
}
