// A declaration whose value names itself by qualified name is a cyclic feature
// value: the read reports the cycle, naming the declaration it closes on, rather
// than recursing without end.
package test {
	private import ScalarValues::*;

	part self {
		attribute a : Real = test::self::a;
	}

	constraint positive { test::self::a > 0.0 }
}
