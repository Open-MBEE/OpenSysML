// The mirror of state_composite_region_depth_order: the region declared first is
// the one whose active state is nested deeper, and it still reacts first.
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working parallel {
			state left {
				entry; then lstart;
				state lstart;
				state deep parallel {
					state inner {
						entry; then dstart;
						state dstart;
						state d1;
						state d2;

						succession first dstart then d1;
						transition first d1 when Tick do assign log := log * 10 + 1 then d2;
					}
				}

				succession first lstart then deep;
			}

			state right {
				entry; then rstart;
				state rstart;
				state r1;
				state r2;

				succession first rstart then r1;
				transition first r1 when Tick do assign log := log * 10 + 2 then r2;
			}
		}

		succession first start then Working;
	}
}
