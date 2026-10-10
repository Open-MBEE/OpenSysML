package repl

import "testing"

// boundChecks declares a requirement and a constraint whose conditions read `in`
// parameters, so a check of either needs arguments for them.
const boundChecks = `package Bound {
    private import ScalarValues::*;
    part def Thing { attribute v : Integer = 2; }
    part t : Thing;
    requirement def Under {
        subject s : Thing;
        in limit : Integer;
        in slack : Integer = 0;
        require constraint { s.v + slack < limit }
    }
    constraint def Between {
        in low : Integer;
        in high : Integer = 10;
        low < high
    }
}`

func TestCheckArgumentsBindParameters(t *testing.T) {
	s := loadModel(t, boundChecks)
	run(t, s, "%instantiate Bound::t")
	for _, check := range []struct{ line, want string }{
		{"%requirement Bound::Under(limit = 5) Bound::t", "✓ Requirement Bound::Under(limit = 5) satisfied"},
		{"%requirement Bound::Under(5, 4) Bound::t", "✗ Requirement Bound::Under(5, 4) failed"},
		{"%requirement Bound::Under(5, slack = 4) Bound::t", "✗ Requirement Bound::Under(5, slack = 4) failed"},
		{"%constraint Bound::Between(3)", "✓ Constraint Bound::Between(3) passed"},
		{"%constraint Bound::Between(3, high = 1)", "✗ Constraint Bound::Between(3, high = 1) failed"},
		{"%constraint Bound::Between(low = 3, high = 4)", "✓ Constraint Bound::Between(low = 3, high = 4) passed"},
	} {
		wants(t, run(t, s, check.line), check.want)
	}
}

func TestCheckArgumentsAreValidated(t *testing.T) {
	s := loadModel(t, boundChecks)
	run(t, s, "%instantiate Bound::t")
	for _, check := range []struct{ line, want string }{
		{"%requirement Bound::Under(limit = 5, bound = 1) Bound::t", "bound"},
		{"%requirement Bound::Under Bound::t", "limit"},
		{"%constraint Bound::Between(1, 2, 3)", "takes 2 argument(s), got 3"},
		{"%constraint Bound::Between(low = \"x\")", "low"},
		{"%constraint Bound::Between(3", "not closed"},
		{"%constraint Bound::Between(3) nosuch", "nosuch"},
	} {
		out := run(t, s, check.line)
		wants(t, out, check.want)
		rejects(t, out, "passed", "satisfied")
	}
}
