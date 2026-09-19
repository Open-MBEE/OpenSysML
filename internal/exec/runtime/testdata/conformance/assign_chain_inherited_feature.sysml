// The feature a chain writes may be inherited: `probe.tag` is declared by the
// supertype the probe specializes, and the object holds it under that name.
package test {
	private import ScalarValues::*;

	part def Base {
		attribute tag : Real = 0.0;
	}

	part def Probe :> Base {
		attribute own : Real = 0.0;
	}

	part probe : Probe;

	action write {
		attribute seen : Real = 0.0;

		first start;

		action set {
			assign probe.tag := 6.5;
		}

		action read {
			assign seen := probe.tag;
		}

		done;

		succession first start then set;
		succession first set then read;
		succession first read then done;
	}
}
