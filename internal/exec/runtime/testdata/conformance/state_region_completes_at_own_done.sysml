// A region completes at its own `done`, not at the `done` of a composite nested in
// it: `C`'s body reaching `done` completes `C`'s body while `C` stays active for its
// do behavior, so region `r3` — and with it `S1` — is not complete once `r2` is.
// `S1` completes only after `C` leaves for `r3`'s `done`, which `Never` lets it.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then S1;
		state S1 parallel {
			state r2 {
				entry; then x;
				state x;
				transition first x accept A then done;
			}
			state r3 {
				entry; then C;
				state C {
					do action wait {
						first start;
						then action w accept Never;
						then action mark assign log := log + "waited ";
					}
					entry; then c1;
					state c1;
					transition first c1 accept B then done;
				}
				transition first C do assign log := log + "C-done " then done;
			}
		}
		transition first S1 do assign log := log + "S1-done " then fin;
		state fin;
	}
	attribute def A;
	attribute def B;
	attribute def Never;
}
