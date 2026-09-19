// A branch of a choice reached from inside an orthogonal region that leaves the
// region set exits every region, in declaration order, and the machine ends with
// a single active state. The regions sit inside `working`, since the choice and
// the state it branches to lie outside the region set. The choice's guard is read
// once the regions are left, so it sees both exit behaviors' writes.
package Test {
	state def RegionExitPseudostate {
		attribute exits : Integer = 0;

		entry; then working;

		state working parallel {
			state left {
				entry; then lstart;
				state lstart;
				state lwork { exit { assign exits := exits + 1; } }

				succession first lstart then lwork;
				transition first lwork then leave;
			}

			state right {
				entry; then rstart;
				state rstart;
				state rwork { exit { assign exits := exits + 10; } }

				succession first rstart then rwork;
			}
		}

		choice leave;
		state stopped;

		transition first leave if exits == 11 then stopped;
	}
}
