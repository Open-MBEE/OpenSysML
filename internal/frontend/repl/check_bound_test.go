package repl

import (
	"strings"
	"testing"
)

// checkDerivedModel derives y = 2x on a part, its asserted constraint, a constraint,
// a requirement and a requirement redefining an inherited y; y > 10 and x < 1 cannot hold.
const checkDerivedModel = `
package CheckDerived {
    private import ScalarValues::*;

    part p {
        attribute x : Real;
        attribute y : Real = x * 2.0;
        assert constraint c { y > 10.0 and x < 1.0 }
    }

    constraint k {
        attribute x : Real;
        attribute y : Real = x * 2.0;
        y > 10.0 and x < 1.0
    }

    requirement r {
        attribute x : Real;
        attribute y : Real = x * 2.0;
        require constraint { y > 10.0 and x < 1.0 }
    }

    abstract requirement def R {
        attribute x : Real;
        attribute y : Real;
        require constraint { y > 10.0 and x < 1.0 }
    }
    requirement r2 : R {
        attribute :>> y = x * 2.0;
    }

    constraint kd {
        attribute x : Real;
        attribute y : Real default x * 2.0;
        y > 10.0 and x < 1.0
    }
}
`

// boundLiteralModel writes limit with `=`, `default` and `:=`, directly, inherited and
// redefined; x > limit and x < 0 hold only where the solver may choose limit.
const boundLiteralModel = `
package BoundLiteral {
    private import ScalarValues::*;

    requirement bound {
        attribute limit : Real = 1.0;
        attribute x : Real;
        require constraint { x > limit and x < 0.0 }
    }
    requirement defaulted {
        attribute limit : Real default 1.0;
        attribute x : Real;
        require constraint { x > limit and x < 0.0 }
    }
    requirement initial {
        attribute limit : Real := 1.0;
        attribute x : Real;
        require constraint { x > limit and x < 0.0 }
    }

    requirement def Bounded {
        attribute limit : Real = 1.0;
        attribute x : Real;
        require constraint { x > limit and x < 0.0 }
    }
    requirement inherited : Bounded;
    requirement def Defaulted {
        attribute limit : Real default 1.0;
        attribute x : Real;
        require constraint { x > limit and x < 0.0 }
    }
    requirement rebound : Defaulted {
        attribute :>> limit = 1.0;
    }
    requirement redefaulted : Defaulted {
        attribute :>> limit default 2.0;
    }
}
`

// TestCheckAssertsADerivedValueOnConstraintsAndRequirements: the defining equation is
// asserted whatever element is checked, one a redefinition supplies included; a derived `default` is free.
func TestCheckAssertsADerivedValueOnConstraintsAndRequirements(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, checkDerivedModel)
	for _, tc := range []struct{ name, verdict, equation string }{
		{"CheckDerived::p", "✗ Part p is unsatisfiable", "CheckDerived::p::y = x * 2.0"},
		{"CheckDerived::p::c", "✗ Constraint c is unsatisfiable", "CheckDerived::p::y = x * 2.0"},
		{"CheckDerived::k", "✗ Constraint k is unsatisfiable", "CheckDerived::k::y = x * 2.0"},
		{"CheckDerived::r", "✗ Requirement r is unsatisfiable", "CheckDerived::r::y = x * 2.0"},
		{"CheckDerived::r2", "✗ Requirement r2 is unsatisfiable", "CheckDerived::r2::y = x * 2.0"},
	} {
		out := run(t, s, "%check "+tc.name)
		wants(t, out, tc.verdict)
		if strings.Contains(out, "= 11.0") {
			t.Errorf("%%check %s left the derived value free:\n%s", tc.name, out)
		}
	}
	wants(t, run(t, s, "%check CheckDerived::kd"), "✓ Constraint kd is satisfiable", "CheckDerived::kd::y = ")
}

// TestCheckFixesAValueBoundWithEqualsAndLeavesADefaultFree: a literal written with `=`
// is fixed as declared; one written with `default` or `:=` is the solver's to choose.
func TestCheckFixesAValueBoundWithEqualsAndLeavesADefaultFree(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, boundLiteralModel)
	bound := run(t, s, "%check BoundLiteral::bound")
	wants(t, bound, "✗ Requirement bound is unsatisfiable")
	explain := run(t, s, "%explain BoundLiteral::bound")
	wants(t, explain, "BoundLiteral::bound::limit == 1.0, declared by the model")
	for _, name := range []string{"BoundLiteral::defaulted", "BoundLiteral::initial"} {
		out := run(t, s, "%check "+name)
		wants(t, out, "is satisfiable", "::limit = ", "::x = ")
	}
}

// TestCheckFixesAnInheritedValueBoundWithEquals: whether a value binds is read from
// the declaration that wrote it, inherited or redefining; a redefined default stays free.
func TestCheckFixesAnInheritedValueBoundWithEquals(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, boundLiteralModel)
	wants(t, run(t, s, "%check BoundLiteral::inherited"), "✗ Requirement inherited is unsatisfiable")
	wants(t, run(t, s, "%check BoundLiteral::rebound"), "✗ Requirement rebound is unsatisfiable")
	wants(t, run(t, s, "%check BoundLiteral::redefaulted"), "✓ Requirement redefaulted is satisfiable", "::limit = ")
}

// shadowedModel: the constraint's own x is bound with `=` beside the part's x of
// the same name, a default; its w is the part's, bound with `=`.
const shadowedModel = `
package Shadow {
    private import ScalarValues::*;

    part h {
        attribute x : Real default 2.0;
        attribute w : Real = 3.0;

        constraint k {
            attribute x : Real = 1.0;
            x > 1.5 and w > 0.0
        }

        constraint m {
            attribute w : Real default 0.0;
            w > 0.0 and w < 1.0
        }
    }
}
`

// TestCheckFixesAValueNamedLikeItsOwners: a feature bound on the constraint is fixed though the
// owning part declares a default under its name; a default on the constraint stays free.
func TestCheckFixesAValueNamedLikeItsOwners(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, shadowedModel)
	wants(t, run(t, s, "%check Shadow::h::k"), "✗ Constraint k is unsatisfiable")
	wants(t, run(t, s, "%explain Shadow::h::k"), "Shadow::h::k::x == 1.0, declared by the model")
	wants(t, run(t, s, "%check Shadow::h::m"), "✓ Constraint m is satisfiable", "Shadow::h::m::w = ")
}

// shadowedObjectModel: the constraint's own x, bound with `=`, is satisfiable
// where the part's x of the same name, the one its objects hold, is not.
const shadowedObjectModel = `
package Shadow {
    private import ScalarValues::*;

    part h {
        attribute x : Real default 2.0;
        attribute w : Real = 3.0;

        constraint k {
            attribute x : Real = 1.0;
            x < 1.5 and w > 0.0
        }
    }
}
`

// TestSolveReadsAnObjectForItsOwnFeaturesOnly: over an object of the part, the
// constraint's own x is fixed as declared, not to the x the object holds.
func TestSolveReadsAnObjectForItsOwnFeaturesOnly(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, shadowedObjectModel)
	run(t, s, "%instantiate Shadow::h")
	wants(t, run(t, s, "%solve Shadow::h::k"), "✓ Constraint k has values satisfying it",
		"Shadow::h::k::x = 1.0  (declared)", "Shadow::h::w = 3  (held by object 1)")
}
