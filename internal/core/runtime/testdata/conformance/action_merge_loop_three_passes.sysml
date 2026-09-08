package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): every arrival traverses the
	// merge, so the loop re-enters `again` once per pass and its body counts each
	// traversal: three passes of `work`, three merge performances, `count = 3`.
	action countTo3 {
		attribute count : Integer = 0;
		attribute merged : Integer = 0;

		first start;
		then merge again { assign merged := merged + 1; }
		then action work { assign count := count + 1; }
		then decide;
			if count < 3 then again;
			if count >= 3 then done;
	}
}
