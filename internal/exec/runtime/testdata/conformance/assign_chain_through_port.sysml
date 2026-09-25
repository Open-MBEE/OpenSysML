// A port is an occurrence with features of its own, so a chain reaching one
// writes the port's feature: `link.level` is written on the port the part owns.
package test {
	private import ScalarValues::*;

	port def Signal {
		attribute level : Real = 0.0;
	}

	part def Rig {
		port link : Signal;

		exhibit state run {
			entry; then driving;

			state driving {
				entry action set {
					assign link.level := 1.5;
				}
			}
		}
	}
}
