// A region written as a state definition inside a parallel body, used by a
// typed usage beside it.
package test {
	state def Machine {
		entry; then running;
		state running parallel {
			state def LeftRegion {
				entry; then l1;
				state l1;
			}
			state left : LeftRegion;

			state def RightRegion {
				entry; then r1;
				state r1;
			}
			state right : RightRegion;
		}
	}
}
