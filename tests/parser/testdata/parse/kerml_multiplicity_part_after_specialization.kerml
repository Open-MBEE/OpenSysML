// FeatureSpecializationPart (KerML.xtext:574) lets a MultiplicityPart follow
// the specializations that stand where the name would go, so `ordered` and
// `nonunique` after `:>> values` continue the part rather than naming it.
package MultiplicityPartAfterSpecialization {
	private import ScalarValues::Real;

	classifier Base {
		feature values : Real[*] nonunique;
		feature more : Real[*] nonunique;
	}
	classifier Sub :> Base {
		feature :>> values ordered : Real;
		feature :>> more ordered nonunique;
	}
	classifier Sub2 :> Base {
		feature :>> values nonunique ordered;
		feature :>> more [1..*] nonunique ordered :> values;
	}
	classifier Sub3 :> Base {
		feature :>> values [5] ordered;
		:>> more [2] nonunique ordered;
	}
	classifier Sub4 :> Base {
		feature :> values ordered;
		feature ::> more nonunique;
		feature : Real [2] nonunique ordered :> values;
		feature redefines more ordered nonunique : Real;
		feature subsets values nonunique;
	}
}
