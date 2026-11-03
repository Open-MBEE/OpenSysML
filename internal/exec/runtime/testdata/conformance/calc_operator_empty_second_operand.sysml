// A binary `+` or `-` is the invocation of the library's `'+'`/`'-'` (KerML
// 1.1 §8.4.4.9.6), whose second parameter `y` is [0..1]: an empty second
// operand — null, `()`, a feature holding no value — makes the operator the
// unary sign, as the function form already is, for every numeric type the
// library declares them on. The first parameter and every operand of the other
// operators are [1], so an empty one there stays refused.
package test {
	private import ScalarValues::*;
	private import NumericalFunctions::*;

	calc def Probe {
		in n : Integer;
		in none : Integer[0..1];
		out opPlusNull : Integer = 1 + null;
		out fnPlusNull : Integer = '+'(1, null);
		out opMinusNull : Integer = 3 - null;
		out fnMinusNull : Integer = '-'(3, null);
		out opMinusEmpty : Integer = 3 - ();
		out fnMinusEmpty : Integer = '-'(3, ());
		out opMinusFeature : Integer = n - none;
		out opNaturalPlus : Natural = 2 + null;
		out opRationalMinus : Rational = 3/4 - null;
		out opRealMinus : Real = 1.5 - null;
		out opRealPlusFn : Real = RealFunctions::'+'(1.5, null);
		return : Integer = n;
	}
	calc probe : Probe { in n = 5; }

	attribute nullPlus : Integer = null + 1;
	attribute fnNullPlus : Integer = NumericalFunctions::'+'(null, 1);
	attribute timesNull : Integer = 2 * null;
	attribute divNull : Rational = 2 / null;
	attribute modNull : Integer = 7 % null;
	attribute powNull : Integer = 2 ** null;
	attribute lessNull : Boolean = 1 < null;
}
