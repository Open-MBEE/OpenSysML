// Two declarations whose values name each other by qualified name, one of them
// through a redefinition, close a cycle: the read reports it, naming the
// declaration the read started from.
package test {
	private import ScalarValues::*;

	part def P { attribute b : Real; }
	part self : P {
		attribute a : Real = test::self::b;
		attribute :>> b = test::self::a;
	}

	constraint positive { test::self::a > 0.0 }
}
