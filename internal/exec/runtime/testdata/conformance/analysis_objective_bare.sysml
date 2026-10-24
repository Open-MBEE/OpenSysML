// An objective stating nothing — no `require`, no `verify` — is a requirement
// check over no constraints, which is true (Requirements::
// RequirementConstraintCheck), so it is satisfied rather than undecided and the
// case's outputs stand on their own.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
	}
	part ship : Ship;

	analysis costed {
		subject s = ship;
		objective obj;
		out c : Real = s.cost;
	}
}
