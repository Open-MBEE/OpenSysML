// The statements of a constraint body are steps of one Boolean performance,
// run in declaration order before its conditions are evaluated in the state
// they left: locals live in that performance alone, the constraint's own
// parameters are copied into it, and a nested body keeps its own.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	constraint def SumBelow {
		in xs : Integer[*];
		attribute total : Integer = 0;
		attribute i : Integer = 0;
		while i < size(xs) { assign total := total + xs#(i + 1); assign i := i + 1; }
		total > 3
	}

	requirement def Safing {
		require constraint { attribute m : Real = 0; assign m := 9; m > 4 }
		assume constraint { attribute b : Boolean = true; assign b := false; not b }
	}

	part def Vehicle {
		attribute z : Real = 1.0;
		constraint assigned { attribute y : Real = 0; assign y := 5; y > 3 }
		constraint branched { attribute y : Real = 0; if z > 0 { assign y := 7; } else { assign y := 1; } y > 3 }
		constraint summed : SumBelow { in xs = (2, 4); }
		// z names a local of the performance, not the part's feature it shadows.
		constraint shadowed { attribute z : Real = 0; assign z := 5; z > 3 }
		assert not constraint { attribute w : Real = 0; assign w := 5; w > 100 }
		requirement safing : Safing;
		requirement fitted {
			require constraint { attribute k : Real = 0; assign k := 7; k > 4 }
		}
	}

	part vehicle : Vehicle;
}
