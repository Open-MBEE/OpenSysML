// A cross-region transition leaves the regions holding neither endpoint alone:
// they are concurrent subperformances of the enclosing StatePerformance and
// nothing in KerML's semantic library terminates them.
package Test {
	state def TransitionCrossRegionThirdRegion {
		attribute crossed : Integer = 0;
		attribute thirdEntries : Integer = 0;
		attribute thirdExits : Integer = 0;

		entry; then start;
		state start;
		state running parallel {
			state left {
				entry; then lstart;
				state lstart;
				state lidle;

				succession first lstart then lidle;
				transition first lidle if crossed == 0 then rtarget;
			}
			state right {
				entry; then rstart;
				state rstart;
				state ridle;
				state rtarget { entry { assign crossed := 1; } }

				succession first rstart then ridle;
			}
			state third {
				entry; then tstart;
				state tstart;
				state tidle {
					entry { assign thirdEntries := thirdEntries + 1; }
					exit { assign thirdExits := thirdExits + 1; }
				}

				succession first tstart then tidle;
			}
		}

		succession first start then running;
	}
}
