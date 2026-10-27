package InheritedPrefixViews {
	private import Views::*;

	port def APort;
	part def Engine;

	part def APart {
		port aPort : APort;
		attribute mass;
		part engine : Engine;
	}

	// Subclassification: BPart has APart's members; the mass it redefines is its own.
	part def BPart :> APart {
		attribute :>> mass = 1;
		part cooler : Engine;
	}

	// Typing: a bodiless usage shows what its definition declares.
	part part1 : APart;

	// Subsetting: part2 has part1's members through APart.
	part part2 :> part1;

	// A redefining usage declared in the owner is its own; the rest stay inherited.
	part part3 : APart {
		port :>> aPort;
	}

	part def Rig {
		part a : APart;
		part b : BPart;
		part c :> a;
		part d : APart {
			attribute :>> mass = 2;
		}
		connect a.aPort to d.aPort;
	}

	view part1Tree {
		expose part1;
		render asTreeDiagram;
	}

	view rigTree {
		expose Rig;
		render asTreeDiagram;
	}

	view rigLinks {
		expose Rig;
		attribute depth = 3;
		render asInterconnectionDiagram;
	}
}
