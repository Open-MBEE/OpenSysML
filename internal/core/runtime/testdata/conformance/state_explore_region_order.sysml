package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): one Go enables a
	// transition in each region of the parallel state `work`. Each fires once,
	// and the library orders neither before the other, so the effects' writes to
	// `last` land in either order; the last one stands.
	state Machine {
		attribute last : Integer = 0;

		entry; then work;
		state work parallel {
			state a {
				entry; then a1;
				state a1;
				state a2;
				transition first a1 accept Go do assign last := 1 then a2;
			}
			state b {
				entry; then b1;
				state b1;
				state b2;
				transition first b1 accept Go do assign last := 2 then b2;
			}
		}
	}
}
