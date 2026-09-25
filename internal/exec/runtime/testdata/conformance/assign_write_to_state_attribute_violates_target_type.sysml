// An attribute owned by a state usage lives in that state's own frame, and a
// write into that frame answers to the attribute's declaration as every other
// write does.
package test {
	private import ScalarValues::*;

	state def Gauge {
		attribute reading : Real = 0.0;
		attribute raw;

		entry; then go;
		state go {
			entry action set {
				assign raw := "not a number";
				assign reading := raw;
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
