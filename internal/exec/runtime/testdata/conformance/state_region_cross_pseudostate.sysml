// A branch that enters a sibling region of the same composite state exits its
// source only: the composite state is not re-entered, and the region the source
// was active in is left without an active state.
package Test {
	state def RegionCrossPseudostate {
		attribute crossed : Integer = 0;

		entry; then start;
		state start;
		state running parallel {
			state left {
				entry; then lstart;
				state lstart;
				state lidle;

				succession first lstart then lidle;
				transition first lidle if crossed == 0 then cross;
			}
			state right {
				entry; then rstart;
				state rstart;
				state ridle;
				state rtarget { entry { assign crossed := 1; } }

				succession first rstart then ridle;
			}
		}
		choice cross;

		succession first start then running;

		transition first cross if crossed == 0 then rtarget;
	}
}
