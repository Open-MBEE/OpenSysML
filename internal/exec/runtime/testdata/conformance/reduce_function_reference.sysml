// A collection operation's body parameter may be bound to a function the argument
// names instead of a body expression (`->reduce '+'`, KerML 1.0 §7.4.9.6
// FunctionReferenceExpression; the library's own NumericalFunctions::sum0 is
// `collection->reduce '+' ?? zero`): the function is applied to each element, or
// to the accumulator and the element, as the body would be.
package test {
	private import ScalarValues::*;
	private import ScalarFunctions::*;
	private import NumericalFunctions::*;
	private import ControlFunctions::*;
	calc def Add { in a : Integer; in b : Integer; return : Integer = a + b; }
	calc def Neg { in a : Integer; return : Integer = -a; }
	part def Rig {
		attribute sum : Integer = (1, 2, 3)->reduce '+';
		attribute product : Integer = (2, 3, 4)->reduce '*';
		attribute largest : Integer = (4, 9, 2)->reduce max;
		attribute single : Integer = (7)->reduce '+';
		attribute viaCalc : Integer = (1, 2, 3)->reduce Add;
		attribute viaBody : Integer = (1, 2, 3)->reduce {in x; in y; x + y};
		attribute negated : Integer[*] = (1, 2, 3)->collect Neg;
		attribute summed : Integer = sum0((1, 2, 3), 0);
	}
}
