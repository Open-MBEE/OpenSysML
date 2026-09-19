package test {
	private import ScalarValues::*;

	// A value whose type the analyzer cannot name is judged where it is known:
	// the run time refuses the String an untyped feature came to hold and leaves
	// the Real attribute as it was.
	part def Rig {
		attribute reading : Real = 0.0;
		attribute raw;

		exhibit state run {
			entry; then go;

			state go {
				entry action set {
					assign raw := "not a number";
					assign reading := raw;
				}
			}
		}
	}
}
