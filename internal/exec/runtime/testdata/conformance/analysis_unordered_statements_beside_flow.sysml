package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): a case body stating a
	// flow performs the assignment no succession reaches as a subaction beside
	// it, once, not also as a local before the flow: the count is raised once.
	analysis tally {
		attribute count : Integer := 0;
		attribute got : Integer := 0;
		first start;
		then action a { assign got := 5; }
		assign count := count + 1;
		return : Integer = count * 10 + got;
	}
}
