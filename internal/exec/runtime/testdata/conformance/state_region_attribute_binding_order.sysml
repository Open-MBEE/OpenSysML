// A state's bound attribute follows the attribute of the region around it, a usage in
// a parallel state, though lowering numbers the region after the states inside it.
package test {
	private import ScalarValues::*;

	state def Inner {
		attribute level : Integer = 1;

		entry; then go;
		state go {
			attribute shown : Integer = level;
			entry action set {
				assign level := 5;
			}
		}
		state far;
		state near;

		transition written first go if shown == 5 then far;
		transition unwritten first go if shown == 1 then near;
	}

	state def Gauge {
		entry; then start;
		state start;

		state working parallel {
			state inside : Inner;
		}

		succession first start then working;
	}

	part def Rig {
		exhibit state run {
			entry; then gauge;
			state gauge : Gauge;
		}
	}
}
