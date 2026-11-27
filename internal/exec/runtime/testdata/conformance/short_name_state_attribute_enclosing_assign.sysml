// A short name written in a state whose own attribute takes the name writes the
// enclosing state's attribute it names, and the guards read that write.
package test {
	private import ScalarValues::*;

	state def Gauge {
		attribute <r> reading : Integer = 1;

		entry; then go;
		state go {
			attribute reading : Integer = 100;
			entry action set {
				assign r := 5;
			}
		}
		state far;
		state near;

		transition written first go if r == 5 then far;
		transition unwritten first go if r == 1 then near;
	}

	part def Rig {
		exhibit state run {
			entry; then gauge;
			state gauge : Gauge;
		}
	}
}
