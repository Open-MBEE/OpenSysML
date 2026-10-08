package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): two unordered composite
	// subactions write one feature; the library orders neither write before the
	// other, so the one that stands is open and both values are admissible.
	action race {
		attribute c : Integer := 0;
		action a { assign c := 1; }
		action b { assign c := 2; }
	}
}
