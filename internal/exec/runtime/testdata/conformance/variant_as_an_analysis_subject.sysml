package test {
	private import ScalarValues::Real;
	part def Body { attribute m : Real; }
	variation part def Case :> Body {
		variant part big : Case { attribute :>> m = 2.0; }
		variant part small : Case { attribute :>> m = 1.0; }
	}
	part def Configuration { part target : Body; }
	part bigConfiguration : Configuration { part :>> target = Case::big; }
	analysis def BodyMass {
		subject body : Body;
		return mass : Real = body.m;
	}
	analysis viaVariant : BodyMass { subject :>> body = Case::big; }
	analysis viaConfiguration : BodyMass { subject :>> body = bigConfiguration.target; }
}
