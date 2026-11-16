// An ordered assertion that fails where the flow reaches it is reported not
// satisfied even though a later step makes its condition true: `drained` is
// judged before `drain` runs. The flow moves on, so the case still reports its
// result, and the unordered `settled` is judged once the body has completed.
package test {
	private import ScalarValues::*;

	analysis def Tank {
		attribute level : Integer := 0;
		action fill { assign level := 3; }
		then assert constraint drained { level == 1 }
		then action drain { assign level := 1; }
		assert constraint settled { level == 1 }
		return remaining : Integer = level;
	}

	// The usage inherits the flow, the assertion at its inherited position.
	analysis tank : Tank;
}
