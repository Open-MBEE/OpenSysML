// A short name names the feature it is declared on (KerML 1.0 §7.3.4.5), and a
// redefining feature is the one feature its redefined name now denotes (KerML
// 1.0 §7.3.4.6): a write through `x` is read back through `sx`, and a write
// through `y` through `base`, each resolving to the one feature the performance
// holds rather than to the declaration's default.
package test {
	private import ScalarValues::*;

	action def Base {
		attribute base : Integer = 0;
	}

	action aliased : Base {
		attribute <sx> x : Integer = 0;
		attribute y :>> base;
		attribute seenX : Integer = -1;
		attribute seenBase : Integer = -1;

		first start;
		then action wx { assign x := 1; }
		then action rx { assign seenX := sx; }
		then action wy { assign y := 1; }
		then action rb { assign seenBase := base; }
		then done;
	}
}
