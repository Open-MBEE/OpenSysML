// A returned usage that binds no value leaves the result parameter unbound, so
// invoking the calc reports no value, as an unassigned `out` does, and says how
// to state a result rather than yielding a made-up one.
package test {
	private import ScalarValues::*;

	calc selectEngine {
		in engine : Real;
		return selectedEngine :> engine;
	}
}
