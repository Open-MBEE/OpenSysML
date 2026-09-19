// A branch of an `if` is a block, so it too states a flow of its own: the taken
// branch reaches its nested action declaration and its `perform` in the order
// they were written, and the branch not taken reaches neither. Here the `then`
// branch runs, doubling 4 to 8 and then performing Halve on it, whose halved
// of 4 is reported under the performing node's path since no feature around
// it holds the name; the else branch runs not at all, leaving untaken at 0.
// Halve's output is Rational because integer division returns one
// (IntegerFunctions).
package test {
	private import ScalarValues::*;

	action run {
		attribute value : Integer = 4;
		attribute result : Integer = 0;
		attribute untaken : Integer = 0;

		first start;

		action choose {
			if value > 3 {
				action double {
					assign result := value * 2;
				}
				perform Halve;
			} else {
				action never {
					assign untaken := untaken + 1;
				}
				perform Halve;
			}
		}

		done;

		succession first start then choose;
		succession first choose then done;
	}

	action def Halve {
		in result : Integer;
		out halved : Rational;

		first begin;

		action compute {
			assign halved := result / 2;
		}

		done;

		succession first begin then compute;
		succession first compute then done;
	}
}
