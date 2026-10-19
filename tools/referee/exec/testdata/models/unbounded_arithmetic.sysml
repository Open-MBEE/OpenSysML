// `*` as an operand: KerML 1.0 §8.3.4.8.11 types a LiteralInfinity as a Positive and
// §8.2.5.8.4 places it above every integer; §9.4 defines no arithmetic result over it.
// Each attribute binds one probe so the pilot can read it as a model-level value.
package Unbounded {
	private import ScalarValues::*;
	private import NaturalFunctions::*;
	attribute add1 : Integer = * + 1;
	attribute add2 : Integer = 1 + *;
	attribute addReal : Real = * + 1.5;
	attribute sub1 : Integer = * - 1;
	attribute sub2 : Integer = 1 - *;
	attribute subBoth : Integer = * - *;
	attribute mul1 : Integer = 2 * *;
	attribute mul0 : Integer = * * 0;
	attribute div1 : Rational = * / 2;
	attribute div2 : Rational = 2 / *;
	attribute mod1 : Integer = * % 2;
	attribute pow1 : Integer = * ** 2;
	attribute pow2 : Integer = 2 ** *;
	attribute caret : Integer = * ^ 2;
	attribute neg : Integer = -*;
	attribute pos : Integer = +*;
	attribute gt : Boolean = * > 1;
	attribute lt : Boolean = * < 1;
	attribute ge : Boolean = * >= 3;
	attribute le : Boolean = * <= *;
	attribute eq : Boolean = * == *;
	attribute eqOne : Boolean = * == 1;
	attribute ne : Boolean = * != 1;
	attribute gtFn : Boolean = '>'(*, 1);
	attribute eqFn : Boolean = '=='(*, 1);
	attribute mx : Integer = max(*, 1);
	attribute mn : Integer = min(*, 1);
	attribute ab : Natural = IntegerFunctions::abs(*);
	attribute str : String = ToString(*);
	attribute rng : Integer[0..*] = 1..*;
	attribute sum1 : Integer = IntegerFunctions::sum((1, *));
	attribute cnt : Natural = SequenceFunctions::size((1, *));
	attribute has : Boolean = SequenceFunctions::includes((1, *), *);
	attribute cond : Integer = if * > 1 ? * else 1;
	attribute hi : Natural = *;
	attribute hiPlus : Natural = hi + 1;
	attribute many : Integer[0..hi];
	calc def Same { in x : Integer; return : Integer = x; }
	calc def Bump { in x : Integer; return : Integer = x + 1; }
	calc def Body { return : Integer = * + 1; }
	attribute same : Integer = Same(*);
	attribute bumped : Integer = Bump(*);
	attribute viaCalc : Integer = Body();
}
