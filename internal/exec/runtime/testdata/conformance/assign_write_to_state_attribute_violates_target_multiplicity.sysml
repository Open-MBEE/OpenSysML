// The collection written into a state usage's own frame satisfies the target's
// multiplicity: three values do not fit a feature declared to hold two.
package test {
	private import ScalarValues::*;

	state def Gauge {
		attribute samples : Integer[2] nonunique = (0, 0);
		attribute raw[3] = (1, 2, 3);

		entry; then go;
		state go {
			entry action set {
				assign samples := raw;
			}
		}
	}

	part def Rig {
		exhibit state run {
			entry; then gauge;
			state gauge : Gauge;
		}
	}
}
