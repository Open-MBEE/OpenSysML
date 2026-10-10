// A short name names the feature it is declared on (KerML 1.0 §7.3.4.5): a write
// through `x` is read back through `sx`, resolving to the one feature the
// performance holds rather than to the declaration's default.
package test {
	private import ScalarValues::*;

	action aliased {
		attribute <sx> x : Integer = 0;
		attribute seenX : Integer = -1;

		first start;
		then action wx { assign x := 1; }
		then action rx { assign seenX := sx; }
		then done;
	}
}
