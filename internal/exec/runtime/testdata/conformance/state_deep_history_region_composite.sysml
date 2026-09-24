// A deep history vertex restores a region whose active state is a composite state:
// the state the region was left in is recorded even when the transition leaving it
// started inside that composite state's own region, so the restore re-enters the
// recorded configuration instead of the region's initial state.
package Test {
	private import SI::*;

	state def DeepHistoryRegionComposite {
		attribute deepEntries : Integer = 0;
		attribute startEntries : Integer = 0;
		attribute trips : Integer = 0;
		// Counts the region starting at its initial state: the restore must not.
		attribute restarts : Integer = 0;

		entry; then start;
		state start;
		state running parallel {
			deep history resume;

			state left {
				entry; then ls;
				state ls;
				state lidle;

				succession first ls then lidle;
			}
			state right {
				entry; then rstart;
				state rstart;
				state wrapper parallel {
					state inner {
						entry; then istart;
						state istart;
						state deepState {
							entry { assign deepEntries := deepEntries + 1; }
						}

						succession first istart then deepState;
						transition out_deep
							first deepState
							accept after 3 [s]
							do { assign trips := trips + 1; }
							then away;
					}
				}

				transition to_wrapper first rstart do { assign restarts := restarts + 1; } then wrapper;
			}
		}
		state away {
			entry { assign startEntries := startEntries + 1; }
		}

		succession first start then running;
		transition back first away accept after 3 [s] if trips == 1 then resume;
	}
}
