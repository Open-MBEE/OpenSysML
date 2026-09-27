// A calc the run gate compiles: -compile drives an external compiler, so on a
// WebAssembly build it must refuse naming that, not fail on the platform's own
// report of the pipes it could not make.
package calcdemo {
	private import ScalarValues::*;

	calc def Square {
		in v : Real;
		return : Real = v * v;
	}
}
