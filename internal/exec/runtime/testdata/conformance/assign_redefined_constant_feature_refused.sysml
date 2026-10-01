// A feature redefining a constant one is constant too: a variable feature
// subsetting or redefining a constant one must be constant (KerML 1.0 §8.3.3.3,
// validateSubsettingConstantConformance). Writing it is refused naming the
// feature it is constant by.
package test {
	private import ScalarValues::*;

	action def Tune {
		constant attribute limit : Integer = 5;
	}

	action tune : Tune {
		attribute :>> limit = 5;

		first start;

		action raise {
			assign limit := 7;
		}

		done;

		succession first start then raise;
		succession first raise then done;
	}
}
