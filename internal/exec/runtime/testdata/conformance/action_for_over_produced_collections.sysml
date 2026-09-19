// A `for` loop iterates every collection the expression layer produces, each in
// its own order: a range ascends (1+2+3 = 6), a filter keeps the order of the
// collection it filtered (2+3 = 5), a collection-valued calculation's result is
// iterated in the order it returned it (2+3 = 5), and a collection with no
// element — an empty sequence, a descending range — is iterated not at all.
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def Above {
		in n : Integer;
		return : Integer[*] = (1..n)->select { in i; i > 1 };
	}

	action run {
		attribute xs : Integer[*] = (1, 2, 3);
		attribute nothing : Integer[*] = ();

		attribute ranged : Integer = 0;
		attribute filtered : Integer = 0;
		attribute returned : Integer = 0;
		attribute empties : Integer = 0;
		attribute descended : Integer = 0;

		first start;

		action iterate {
			for i in 1..3 {
				assign ranged := ranged + i;
			}
			for x in xs->select { in y; y > 1 } {
				assign filtered := filtered + x;
			}
			for c in Above(3) {
				assign returned := returned + c;
			}
			for e in nothing {
				assign empties := empties + 1;
			}
			for d in 3..1 {
				assign descended := descended + 1;
			}
		}

		done;

		succession first start then iterate;
		succession first iterate then done;
	}
}
