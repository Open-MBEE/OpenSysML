// An action's `out` parameter is a feature of its performance, so it is readable
// through the feature chain of the perform usage once the action has run.
package test {
	private import ScalarValues::*;

	action def Measure {
		out attribute n : Integer;

		action sample {
			assign n := 42;
		}
		first sample;
	}

	part def Probe {
		perform action reading : Measure;
	}
}
