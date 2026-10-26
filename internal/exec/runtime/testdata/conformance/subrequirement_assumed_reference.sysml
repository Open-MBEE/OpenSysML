// A requirement assumed by reference brings its subrequirements along as
// assumptions: spec requires nothing, so it holds vacuously without evaluating
// child's `missing > 0`, whose input nothing binds.
package test {
	private import ScalarValues::*;
	requirement def R {
		attribute missing : Real;
		requirement child { require constraint { missing > 0.0 } }
	}
	requirement r0 : R;
	requirement spec { assume r0; }
}
