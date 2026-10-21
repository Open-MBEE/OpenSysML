// A parameter declared [0..*] holds one bound value as a one-element sequence;
// a condition, an identity and an equality read that value through the
// sequence, as every operator does, so the body computes as it would on the
// bare value.
package test {
	private import ScalarValues::*;

	calc def Count { in x : Boolean[0..*]; attribute n : Integer = 0; while x and n < 3 { assign n := n + 1; } return : Integer = n; }
	calc def Pick { in x : Boolean[0..*]; return : Integer = if x ? 1 else 2; }
	calc def IsOne { in x : Integer[0..*]; return : Boolean = x === 1; }
	calc def EqOne { in x : Integer[0..*]; return : Boolean = x == 1; }

	part def Host {
		attribute counted : Integer = Count(true);
		attribute picked : Integer = Pick((false));
		attribute isOne : Boolean = IsOne((1));
		attribute isOneBare : Boolean = IsOne(1);
		attribute isOnePair : Boolean = IsOne((1, 2));
		attribute eqOne : Boolean = EqOne((1));
		attribute eqOneBare : Boolean = EqOne(1);
	}
}
