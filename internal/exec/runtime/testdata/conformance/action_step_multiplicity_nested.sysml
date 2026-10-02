package test {
	private import ScalarValues::*;

	action def Nested {
		attribute c : Integer = 0;
		first start then outer;
		action outer {
			first start then inner;
			action inner[2] { assign c := c + 1; }
			then done;
		}
		then done;
	}
}
