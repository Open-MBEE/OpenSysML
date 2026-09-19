// A usage typed only through another usage it subsets, so the definition it is
// of is not among its direct supertypes.
package Fleet {
	part def Rover {
		attribute level = 0;

		exhibit state modes {
			entry; then waiting;
			state waiting;
		}

		action bump {
			first b;
			action b {
				assign level := level + 1;
			}
		}
	}

	part rover : Rover;
	part scout :> rover;
}
