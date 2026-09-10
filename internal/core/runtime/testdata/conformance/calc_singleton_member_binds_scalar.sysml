// A feature's values are a sequence its multiplicity constrains, so a `[0..*]`
// member holding one value denotes that value wherever one is taken: an
// operator operand, a `[1]` calc parameter, a cast or a classification operand.
package test {
	private import ScalarValues::*;
	private import RealFunctions::sqrt;
	private import ComplexFunctions::*;
	private import QuantityCalculations::isZero;
	private import ISQ::*;
	private import SI::*;

	calc def Inc { in x : Real; x + 1.0 }

	part def Holder {
		attribute zs : Real[0..*];
		attribute cs : Complex[0..*];
		attribute ls : LengthValue[0..*];
	}

	calc def SingletonMember {
		attribute q : Holder = new Holder(zs = (2.5), cs = (rect(3.0, 4.0)), ls = (2.0 [m]));
		out sum = q.zs + 1.0;
		out less = q.zs < 3.0;
		out same = q.zs == 2.5;
		out incremented = Inc(q.zs);
		out root = sqrt(q.zs);
		out cast = q.zs as Real;
		out classified = q.zs istype Real;
		out negated = -q.zs;
		out realPart = re(q.cs);
		out zeroLength = isZero(q.ls);
	}

	calc c : SingletonMember;
}
