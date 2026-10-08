package test {
	private import ScalarValues::*;

	action def LocalFrames {
		attribute c : Integer = 0;
		first start then a;
		action a[3] {
			attribute l : Integer = 0;
			assign l := l + 1;
			then assign c := c + l;
		}
		then done;
	}
}
