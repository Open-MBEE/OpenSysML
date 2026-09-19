// An operator function's `[1]` operand may arrive as a one-element collection —
// a filtered sequence, say — and stands for that element.
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def SingletonOperands {
		attribute xs : Integer[0..*] = (1, 2, 3);
		attribute largest : Integer[0..*] = xs->select {in i; i > 2};
		out sum = IntegerFunctions::'+'(largest, 1);
		out negated = IntegerFunctions::'-'(largest);
		out less = RealFunctions::'<'(largest, 3.5);
		out quotient = NaturalFunctions::'/'(6, largest);
		out negation = BooleanFunctions::'not'(xs->exists {in i; i > 2});
		out smallest = ScalarFunctions::min(largest, 5);
	}

	calc c : SingletonOperands;
}
