// FeatureSpecializationPart (SysML.xtext:366) lets a MultiplicityPart follow
// the specializations that stand where the name would go, so `ordered` and
// `nonunique` after `:>> values` continue the part rather than naming it.
package MultiplicityPartAfterSpecialization {
	private import ScalarValues::Real;

	attribute def Base {
		attribute values : Real[*] nonunique;
		attribute more : Real[*] nonunique;
	}
	attribute def Sub :> Base {
		:>> values ordered : Real;
		:>> more ordered nonunique;
	}
	attribute def Sub2 :> Base {
		:>> values nonunique ordered;
		attribute :>> more [1..*] nonunique ordered :> values;
	}
	attribute def Sub3 :> Base {
		:>> values [5] ordered = (1, 2, 3, 4, 5);
		attribute :>> more [0..*] ordered nonunique : Real;
	}
	attribute def Sub4 :> Base {
		attribute :> values ordered;
		attribute ::> more nonunique;
		attribute : Real [2] nonunique ordered :> values;
		attribute redefines more ordered nonunique : Real;
		attribute subsets values nonunique;
	}
}
