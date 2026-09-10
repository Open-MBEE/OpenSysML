// A `[0..*]` member holding two values is a sequence; an operator taking one
// value refuses it, as before.
package test {
	private import ScalarValues::*;

	part def Holder { attribute zs : Real[0..*]; }

	calc def TwoValued {
		attribute q : Holder = new Holder(zs = (1.0, 2.0));
		out sum = q.zs + 1.0;
	}

	calc c : TwoValued;
}
