// One Go enables a transition in each region of `work`. Region a's disarms the
// guard of region b's, whose target is a junction drawn once b's transition
// fires; the left branch writes route from nothing and seen from the guard's own
// feature. The guard, read as b's transition fires, races a's write: whether
// either branch effect ran stands on the order, so route stands on it as seen does.
package Test {
	state Machine {
		attribute armed : Boolean = true;
		attribute route : Integer = 0;
		attribute seen : Boolean = false;

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
				transition first split do action both { assign route := 1; assign seen := armed; } then left;
				transition first split do assign route := 2 then right;
			}
		}
	}
}
