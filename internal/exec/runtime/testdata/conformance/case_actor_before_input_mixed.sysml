// Positional and named arguments together: the positional argument binds the
// first `in` parameter after the actor, the named one the input it names, and
// the actor declared before them takes neither.
package test {
	private import ScalarValues::*;

	part def Operator;
	part def Thing {
		attribute mass : Real = 4.0;
	}

	analysis def ScaledMass {
		subject t : Thing;
		actor op : Operator;
		in factor : Real;
		in offset : Real default 0.0;
		return : Real = t.mass * factor + offset;
	}

	part thing : Thing;
}
