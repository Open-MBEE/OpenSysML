// F64: an expression body is a calculation body, so it may declare features of
// its own between its parameters and its result expression.
package BodyDeclarations {
	private import ScalarValues::*;
	private import ControlFunctions::forAll;
	calc def C {
		in radius : Real;
		(1..3)->forAll { in i : Real; private attribute scaled : Real = radius * i; scaled > 0.0 }
	}
}
