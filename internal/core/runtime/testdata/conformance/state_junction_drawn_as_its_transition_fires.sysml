// One Go enables a transition in each region of `work`. Region a's disarms the
// guard of region b's, whose target is a junction with both branches enabled
// when the transitions are selected. The regions fire in an open order: b first
// takes the junction, drawn among its two branches once b's transition fires;
// a first leaves b's guard false, so b stays and nothing is drawn at the junction.
package Test {
	state Machine {
		attribute armed : Boolean = true;
		attribute route : Integer = 0;

		entry; then work;
		state work parallel {
			state a {
				entry; then a1;
				state a1;
				state a2;
				transition first a1 accept Go do assign armed := false then a2;
			}
			state b {
				entry; then b1;
				state b1;
				junction split;
				state left;
				state right;
				transition first b1 accept Go if armed then split;
				transition first split do assign route := 1 then left;
				transition first split do assign route := 2 then right;
			}
		}
	}
}
