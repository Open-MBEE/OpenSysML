package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): both guards hold for
	// level = 75, and the library says only that exactly one branch follows the
	// decision, not which; the executor takes the first declared and reports the
	// choice.
	action route {
		attribute level : Integer = 75;
		attribute handler : Integer = 0;

		first start;
		then decide select;
			if level > 50 then warn;
			if level > 70 then alarm;

		action warn { assign handler := 1; }
		then done;

		action alarm { assign handler := 2; }
		then done;
	}
}
