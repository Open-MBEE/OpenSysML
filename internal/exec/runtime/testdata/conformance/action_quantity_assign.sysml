// An action assignment whose value is a quantity: the assigned value carries
// the unit its operands composed, and a later comparison reads it back in a
// commensurable unit. The units are named qualified because a wildcard import
// is not in scope inside an action body (see the PR body).
package test {
	public import SI::*;

	action travel {
		attribute speed : ScalarValues::Real = 0.0;
		attribute quick : ScalarValues::Boolean = false;

		first start;

		action compute {
			assign speed := 10.0 [SI::m] / 2.0 [SI::s];
		}

		action check {
			assign quick := speed >= 18.0 [SI::km/SI::h];
		}

		done;

		succession first start then compute;
		succession first compute then check;
		succession first check then done;
	}
}
