// FeatureSpecializationPart (KerML.xtext:574) lets a parameter's
// specializations follow its multiplicity, and its `ordered`/`nonunique`.
package ParameterMultiplicityBeforeSpecialization {
	private import ScalarValues::Integer;

	action def In { in x : Integer; }
	action def Act :> In { in x : Integer[1] redefines In::x; }
	action def Act2 :> In { in x : Integer[1] :>> In::x; }
	action def Act3 :> In { in x : Integer redefines In::x; }

	action def Out { out o : Integer[0..1]; }
	action def Act4 :> Out { out o : Integer[1] :>> Out::o; }

	action def InOut { inout io : Integer; }
	action def Act5 :> InOut { inout io : Integer[1] :>> InOut::io; }

	action def Many { in xs : Integer[*] ordered nonunique; }
	action def Act6 :> Many { in xs : Integer[*] ordered nonunique :>> Many::xs; }

	calc def CBase { in a : Integer; return r : Integer[*] ordered; }
	calc def C :> CBase {
		in a : Integer[1] :>> CBase::a;
		return : Integer[1] ordered :>> CBase::r;
	}

	calc def CMany { in bs : Integer[*] ordered; return r : Integer[*] ordered; }
	calc def C2 :> CMany {
		in bs[1] ordered : Integer :>> CMany::bs;
		return r : Integer[1] ordered :>> CMany::r;
	}

	part def A { attribute w : Integer; }
	part def B :> A {
		attribute w : Integer[1] redefines A::w;
		attribute w2 : Integer[1] :> A::w;
	}
}
