// A feature's values are a sequence its multiplicity constrains, so a `[0..*]`
// member holding one value denotes that value wherever one is taken: an
// operator operand, a `[1]` calc parameter, a cast or a classification operand.
package test {
	private import ScalarValues::*;
	private import RealFunctions::sqrt;

	calc def Inc { in x : Real; x + 1.0 }

	part def Holder { attribute zs : Real[0..*]; }

	calc def SingletonMember {
		attribute q : Holder = new Holder(zs = (2.5));
		out sum = q.zs + 1.0;
		out less = q.zs < 3.0;
		out same = q.zs == 2.5;
		out incremented = Inc(q.zs);
		out root = sqrt(q.zs);
		out cast = q.zs as Real;
		out classified = q.zs istype Real;
		out negated = -q.zs;
	}

	calc c : SingletonMember;
}
