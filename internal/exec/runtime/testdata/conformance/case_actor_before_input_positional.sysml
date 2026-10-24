// A case's actors bind by name only: a positional argument skips an actor and
// binds the next `in` parameter in declaration order, so an actor declared
// before the inputs takes no positional slot. The arguments here fill the two
// inputs after the actor; the model's reads invoke a requirement with the same
// shape positionally, by name and with a defaulted input.
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

	requirement def WithinLimit {
		subject v : Real;
		actor op : Operator;
		in max : Real;
		in slack : Real default 0.0;
		require constraint { v < max + slack }
	}

	part thing : Thing;

	attribute positional : Boolean = WithinLimit(12.0, 10.0, 5.0);
	attribute positionalDefaulted : Boolean = WithinLimit(3.0, 10.0);
	attribute positionalViolated : Boolean = WithinLimit(12.0, 10.0);
	attribute named : Boolean = WithinLimit(v = 12.0, max = 10.0, slack = 5.0);
	attribute namedDefaulted : Boolean = WithinLimit(v = 3.0, max = 10.0);
}
