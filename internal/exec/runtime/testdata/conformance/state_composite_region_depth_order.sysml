// Orthogonal regions react to one event in declaration order, whatever their
// nesting depth: the region declared first reacts before a region whose active
// state is nested deeper.
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working parallel {
			state left {
				entry; then lstart;
				state lstart;
				state l1;
				state l2;

				succession first lstart then l1;
				transition first l1 when Tick do assign log := log * 10 + 1 then l2;
			}

			state right {
				entry; then rstart;
				state rstart;
				state deep parallel {
					state inner {
						entry; then dstart;
						state dstart;
						state d1;
						state d2;

						succession first dstart then d1;
						transition first d1 when Tick do assign log := log * 10 + 2 then d2;
					}
				}

				succession first rstart then deep;
			}
		}

		succession first start then Working;
	}
}
