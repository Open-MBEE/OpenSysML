package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): the first guard holds for
	// level = 75; the second has no result (it divides by zero). A guard with no
	// result is not true, so its branch is not selected and the run is unchanged;
	// the executor reports the guard it could not evaluate.
	action route {
		attribute level : Integer = 75;
		attribute handler : Integer = 0;

		first start;
		then decide select;
			if level > 50 then warn;
			if 1 / (level - 75) > 0 then alarm;

		action warn { assign handler := 1; }
		then done;

		action alarm { assign handler := 2; }
		then done;
	}
}
