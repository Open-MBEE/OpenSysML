package StateSingleRegionParallelWrapper {
	private import ScalarValues::*;

	attribute def Step;

	state def Machine {
		attribute entries : Integer = 0;
		attribute exits : Integer = 0;
		entry; then S;

		state S parallel {
			entry { assign entries := entries + 1; }
			exit { assign exits := exits + 1; }

			state R {
				entry; then A;
				state A;
				state B;
				transition first A accept Step then B;
				transition first B then done;
			}
		}

		state Other;
		transition first S then Other;
	}
}
