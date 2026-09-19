package test {
	private import ScalarValues::*;

	// A node stating no flow of its own keeps running its statements as a leaf.
	action trek {
		out attribute legs : Integer;

		first leg;
		action leg {
			assign legs := 1;
			assign legs := legs + 1;
		}
		succession first leg then done;
		done;
	}
}
