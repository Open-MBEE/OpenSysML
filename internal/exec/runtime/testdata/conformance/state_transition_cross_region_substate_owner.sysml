// The state owning the source's region may itself be a substate rather than a
// state of a region: the concurrent region holding the target is then still found
// by walking out to the region its nearest ancestor is declared in, so that region
// exits the state it was running instead of keeping it beside the target.
package Test {
	state def TransitionCrossRegionSubstateOwner {
		attribute lidleExits : Integer = 0;
		attribute deepExits : Integer = 0;
		attribute midExits : Integer = 0;
		attribute wrapperExits : Integer = 0;
		attribute runningEntries : Integer = 0;

		entry; then start;
		state start;
		state running parallel {
			entry { assign runningEntries := runningEntries + 1; }

			state left {
				entry; then ls;
				state ls;
				state lidle { exit { assign lidleExits := lidleExits + 1; } }
				state lstate;

				succession first ls then lidle;
			}
			state right {
				entry; then rs;
				state rs;
				state wrapper {
					exit { assign wrapperExits := wrapperExits + 1; }

					state mid parallel {
						exit { assign midExits := midExits + 1; }

						state inner {
							entry; then is;
							state is;
							state ideep { exit { assign deepExits := deepExits + 1; } }

							succession first is then ideep;
							transition first ideep if lidleExits == 0 then lstate;
						}
					}
				}

				succession first rs then mid;
			}
		}

		succession first start then running;
	}
}
