// One Go enables a transition in each region of `work`. Region b's targets a
// junction with two enabled branches, each leading on to a second junction whose
// guards read `d`, which region a's effect zeroes. Every guard on the route is read
// once, as b's transition is selected, so whichever branch is drawn and whichever
// region fires first, the second junction takes the branch d = 2 enabled then:
// the one dividing by d, whose guard would be unevaluable read against d = 0.
package Test {
	state Machine {
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
				junction again;
				state left;
				state right;
				transition first b1 accept Go then split;
				transition first split do assign route := 1 then again;
				transition first split do assign route := 2 then again;
				transition first again if 4 / d > 1 do assign route := route + 10 then left;
				transition first again if d == 0 do assign route := route + 20 then right;
			}
		}
	}
}
