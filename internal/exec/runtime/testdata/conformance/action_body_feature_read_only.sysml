package test {
	private import ScalarValues::*;

	action run {
		constant attribute locked : Integer = 1;
		first start;
		then action write { assign locked := 2; }
	}
}
