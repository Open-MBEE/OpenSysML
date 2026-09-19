// One Go enables a transition in each region of `work`. Region b's targets a
// junction whose second branch's guard divides by `d`, which region a's effect
// zeroes. The junction's guards are read once, as b's transition is selected,
// so when a fires first the branch drawn for b is the one enabled then: the
// second is taken without its guard being read again against d = 0.
package Test {
	state Machine {
		attribute v : Integer = 4;
		attribute d : Integer = 2;
		attribute route : Integer = 0;

		entry; then work;
		state work parallel {
			state a {
				entry; then a1;
				state a1;
				state a2;
				transition first a1 accept Go do assign d := 0 then a2;
			}
			state b {
				entry; then b1;
				state b1;
				junction split;
				state left;
				state right;
				transition first b1 accept Go then split;
				transition first split if v > 0 do assign route := 1 then left;
				transition first split if v / d > 1 do assign route := 2 then right;
			}
		}
	}
}
