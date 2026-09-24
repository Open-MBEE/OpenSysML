package test {
	private import ScalarValues::*;

	// A keyword after a kind keyword is the declaration's name.
	action flow {
		attribute item : Integer = 1;
	}

	action outer {
		first start;
		action flow {
			assign item := 2;
		}
		done;
		succession first start then flow;
		succession first flow then done;
	}

	// A keyword before a kind keyword qualifies it instead: the declaration's
	// own name follows the kind, or it has none at all.
	item part Shape;
	part def Bound {
		part base { part edges; }
		part boundaryEdges;
		binding [1] bind [0..1] base.edges = [0..1] boundaryEdges;
	}
	individual item def Alice;
	part def Constrained {
		attribute limit : Integer = 3;
		assert constraint {
			limit > 0
		}
	}
}
