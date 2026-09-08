// A composed target keeps a value only where all of its operands do, so an
// operand nothing states about does not hide one the value's own content
// excludes it by, whichever is written first (KerML 1.1 §8.3.4.9).
package test {
	private import ScalarValues::*;

	attribute def Even :> Integer;
	attribute def EvenInteger intersects Even, Integer;
	attribute def IntegerEven intersects Integer, Even;
	attribute def NonIntegralLast differences Real, Even, Integer;
	attribute def NonIntegralFirst differences Real, Integer, Even;

	calc def ComposedCasts {
		out fractionalUndecidedFirst : EvenInteger[0..*] = 5.5 as EvenInteger;
		out fractionalDecidedFirst : IntegerEven[0..*] = 5.5 as IntegerEven;
		out integralSubtractedLast : NonIntegralLast[0..*] = 5.0 as NonIntegralLast;
		out integralSubtractedFirst : NonIntegralFirst[0..*] = 5.0 as NonIntegralFirst;
	}

	calc c : ComposedCasts;
}
