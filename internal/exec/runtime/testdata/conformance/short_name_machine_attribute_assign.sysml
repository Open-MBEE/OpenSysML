// A machine's attribute written through its short name in a state whose own
// attribute takes the name is the machine's, and the guards read that write.
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
}
