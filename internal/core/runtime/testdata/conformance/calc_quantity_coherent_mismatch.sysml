// A computed quantity is admitted to the kind its result declares by its reduced
// dimension, not its spelling: a speed composed as `'m⋅s⁻²' * s` is refused by an
// acceleration result, and the diagnostic names the coherent unit it was reduced to.
// The operands declare no quantity kind, so only the write itself can refuse.
package test {
	private import SI::*;
	private import ISQ::*;

	attribute gravity = 2.5 ['m⋅s⁻²'];
	attribute duration = 2 [s];

	calc def Misdeclared {
		return v : AccelerationValue = gravity * duration;
	}
}
