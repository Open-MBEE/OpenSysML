// A subrequirement that requires nothing is a satisfied check with no required
// constraint, assumed or not: the container holds vacuously (SysML v2 §9.2.14).
// It is not a condition-less constraint, which would bind no result.
package test {
	requirement outer {
		requirement r0;
		assume r0;
	}
}
