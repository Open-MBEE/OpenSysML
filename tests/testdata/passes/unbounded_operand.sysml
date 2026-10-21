// `*` as an operand of an operator or a library function. KerML 1.0 §8.3.4.8.11
// types the literal as a Positive, so every expression below is well typed and
// the checker reports nothing, as the pinned pilot validator reports nothing on
// this fixture. Which of them has a value is the evaluator's to say: the
// ordering §8.2.5.8.4 gives `*` ("greater than all the integers") answers the
// comparisons, equality, max, min and abs; the Kernel Function Library defines
// no arithmetic result over it, so `+`, `-`, `*`, `/`, `%`, `**`, `^`, unary
// `-`, `..`, `sum` and `product` report an undefined value when evaluated, as a
// division by zero does — not a diagnostic the specification does not state.
package UnboundedOperand {
	private import ScalarValues::*;
	private import NaturalFunctions::*;
	attribute add : Integer = * + 1;
	attribute sub : Integer = 1 - *;
	attribute mul : Integer = 2 * *;
	attribute div : Rational = * / 2;
	attribute mod : Integer = * % 2;
	attribute pow : Integer = * ** 2;
	attribute neg : Integer = -*;
	attribute pos : Integer = +*;
	attribute rng : Integer[0..*] = 1..*;
	attribute total : Integer = IntegerFunctions::sum((1, *));
	attribute gt : Boolean = * > 1;
	attribute eq : Boolean = * == *;
	attribute gtFn : Boolean = '>'(*, 1);
	attribute mx : Natural = max(*, 1);
	attribute mn : Natural = min(*, 1);
	attribute ab : Natural = IntegerFunctions::abs(*);
	attribute str : String = ToString(*);
	attribute hi : Natural = *;
	attribute hiPlus : Natural = hi + 1;
	calc def Bump { in x : Integer; return : Integer = x + 1; }
	attribute bumped : Integer = Bump(*);
}
