// An action a part performs reads that part's features by their bare names:
// an owned performance takes the object owning it as its context occurrence,
// so the object's features are in the performance's own scope.
package test {
	private import ScalarValues::*;

	part def Host {
		attribute seen : Integer = 0;

		perform action marking {
			action step {
				assign seen := seen + 2;
			}
			first step;
		}
	}
}
