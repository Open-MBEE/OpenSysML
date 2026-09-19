// A performed action written in the part owning it names that part's features:
// its body is lexically inside the part, so a name the action does not declare
// resolves to the feature of the object performing it, and writes that object.
package test {
	private import ScalarValues::*;

	part def Host {
		attribute seen : Integer = 0;

		perform action marking {
			action step {
				assign seen := seen + 1;
			}
			first step;
		}
	}
}
