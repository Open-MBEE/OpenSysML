// A redefining attribute declaring no multiplicity inherits the bound the
// redefined declaration stated (KerML 1.1 §7.3.4.5, §8.4.4.12.1): under `[1]` it
// holds one value, which the solver encodes as one variable; under `[0..*]` it
// may hold several, which the solver refuses to read as one.
package test {
	private import ScalarValues::Real;

	part def Base {
		attribute level : Real[1];
		attribute readings : Real[0..*];
	}

	part def Derived :> Base {
		attribute :>> level = 2.0;
		attribute :>> readings = (2.0, 3.5);
		constraint levelNonNegative { level >= 0.0 }
		constraint readingsNonNegative { readings >= 0.0 }
	}

	part derived : Derived;
}
