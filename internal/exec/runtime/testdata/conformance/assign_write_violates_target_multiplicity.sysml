package test {
	private import ScalarValues::*;

	// The collection written satisfies the target's multiplicity: three values
	// do not fit a feature declared to hold two, however they were computed.
	part def Source {
		attribute raw[3] = (1, 2, 3);
	}

	part def Rig {
		attribute samples : Integer[2] nonunique = (0, 0);
		part source : Source;

		exhibit state run {
			entry; then go;

			state go {
				entry action set {
					assign samples := source.raw;
				}
			}
		}
	}
}
