// The body of a performed action resolves `gain` to the action's own parameter
// (KerML 1.0 §8.2.3.5.3), never to the same-named attribute of the object
// performing it: `measure` binds nothing to `gain`, so the parameter's default
// is read, and `Sensor::gain` is reached by its owner only.
package test {
	private import ScalarValues::*;

	action def Measure {
		in gain : Integer = 3;
		out result : Integer = 0;

		first start then compute;
		action compute { assign result := gain * 2; }
		first compute then done;
	}

	part def Sensor {
		attribute gain : Integer = 100;
		perform action measure : Measure;
		attribute reading : Integer = measure.result;
		attribute own : Integer = gain;
	}
}
