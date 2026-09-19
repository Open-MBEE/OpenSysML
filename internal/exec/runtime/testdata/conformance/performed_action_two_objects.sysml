// Each object performing the same action definition holds a distinct performance
// occurrence, so the values one writes are not the other's.
package test {
	private import ScalarValues::*;

	action def Count {
		attribute counted : Integer = 0;

		action step {
			assign counted := counted + 1;
		}
		first step;
	}

	part def Node {
		perform action counting : Count;
	}

	part def Pair {
		part a : Node;
		part b : Node {
			perform action :>> counting : Count {
				attribute redefines counted = 40;
			}
		}
	}
}
