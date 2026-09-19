package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): one write of temp
	// raises the change condition of a transition in each region of the parallel
	// state `work`. Each fires once, and the library orders neither before the
	// other, so the effects' writes to `last` land in either order; the last one
	// stands.
	state Machine {
		attribute temp : Integer = 0;
		attribute last : Integer = 0;

		entry; then start;
		state start;
		state work parallel {
			state a {
				entry; then a1;
				state a1;
				state a2;
				transition first a1 accept when temp > 20 do assign last := 1 then a2;
			}
			state b {
				entry; then b1;
				state b1;
				state b2;
				transition first b1 accept when temp > 20 do assign last := 2 then b2;
			}
		}

		transition first start do assign temp := 30 then work;
	}
}
