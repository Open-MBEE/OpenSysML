// A returned usage that binds no value computes nothing, so invoking the calc
// fails with the typed no-return error rather than yielding a made-up result.
package test {
	private import ScalarValues::*;

	calc selectEngine {
		in engine : Real;
		return selectedEngine :> engine;
	}
}
