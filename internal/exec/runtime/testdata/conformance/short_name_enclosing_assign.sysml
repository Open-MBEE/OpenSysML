package test {
	private import ScalarValues::*;

	// A short name written inside a nested action reaches the feature it names, past
	// an action between the two that declares another feature under the long name.
	action outer {
		out attribute far : Integer = 0;
		out attribute near : Integer = 0;
		attribute <s> x : Integer := 1;
		first start;
		then action mid {
			attribute x : Integer := 100;
			first start;
			then action inner { assign s := 5; }
			then assign near := x;
			then done;
		}
		then assign far := x;
		then done;
	}
}
