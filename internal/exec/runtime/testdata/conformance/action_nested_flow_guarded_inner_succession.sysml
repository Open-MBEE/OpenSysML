package test {
	private import ScalarValues::*;

	// A guard inside a node's own flow decides that flow's path, and the node
	// completes when whichever path it took reaches its end.
	action trek {
		attribute battery : Integer = 5;
		out attribute picked : Integer;

		first leg;

		action leg {
			first check;
			action check;
			action low { assign picked := 1; }
			action high { assign picked := 2; }
			succession first check if battery < 10 then low;
			succession first check if battery >= 10 then high;
		}

		succession first leg then done;
		done;
	}
}
