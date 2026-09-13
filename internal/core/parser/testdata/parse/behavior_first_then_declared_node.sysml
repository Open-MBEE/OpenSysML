package FirstNamingADeclaredNode {
	// `first s1 then s2;` parses as one member carrying both ends of the
	// succession s1 -> s2. Lowering reads it as a succession, not as a start, so
	// the two-ended shape here is what it depends on.
	action def Compact {
		action s1;
		action s2;
		first s1 then s2;
	}

	// The one-ended `first s1;` marks the node the flow starts at; the succession
	// leaving it is written apart.
	action def Split {
		action s1;
		action s2;
		first s1;
		succession first s1 then s2;
	}

	// A `first` end naming nothing the body declares is an initial node of its own.
	action def OwnInitialNode {
		action s1;
		first start;
		succession first start then s1;
	}

	// Beside `first start;`, any number of two-ended `first` members are the
	// successions they state, with or without a body.
	action def ManyBesideStart {
		action a;
		action b;
		action c;
		first start;
		then fork f;
			then a;
			then b;
		first a then j;
		first b then j;
		join j;
		first j then c { doc /* c runs last */ }
		first c then done;
	}
}
