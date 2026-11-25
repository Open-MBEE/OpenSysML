// A state's attribute written through its short name answers to the attribute's
// declaration as a write through its name does.
package test {
	private import ScalarValues::*;

	state def Gauge {
		attribute <r> reading : Real = 0.0;

		entry; then go;
		state go {
			entry action set {
				assign r := "not a number";
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
