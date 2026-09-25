// `===` is identity of occurrences, not equality of what they hold: two parts
// of one definition hold equal values yet are two occurrences, in operator and
// in function form alike, and nothing is the same occurrence as nothing.
package test {
	private import ScalarValues::*;

	part def Widget { attribute n : Integer = 1; }

	part ctx {
		part a : Widget;
		part b : Widget;
		part none : Widget[0..1];
		attribute sameSelf : Boolean = a === a;
		attribute sameOther : Boolean = a !== b;
		attribute equalValues : Boolean = a.n == b.n;
		attribute sameByFunction : Boolean = OccurrenceFunctions::'==='(a, a);
		attribute otherByFunction : Boolean = OccurrenceFunctions::'==='(a, b);
		attribute nothingIsNothing : Boolean = OccurrenceFunctions::'==='(none, none);
		attribute nothingIsNotA : Boolean = OccurrenceFunctions::'==='(none, a);
	}
}
