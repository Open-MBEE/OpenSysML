package test {
	private import ScalarValues::*;

	// A calc whose body is a computation — locals and a loop — and whose outputs
	// are features of it. The body runs once; each output is then evaluated
	// against the environment the body left behind, and an output may name the
	// calc's other outputs as well as its locals and parameters.
	calc def Budget {
		in n : Integer;
		attribute doubled : Integer = 0;
		while doubled < n {
			assign doubled := doubled + 2;
		}
		out reached = doubled;
		out margin = reached - n;
		out summary = reached + margin;
		return : Integer = doubled;
	}

	calc b : Budget {
		in n = 7;
	}
}
