package test {
	private import ScalarValues::Real;
	part def Body { attribute m : Real; }
	variation part def Case :> Body {
		variant part big : Case { attribute :>> m = 2.0; }
		variant part small : Case { attribute :>> m = 1.0; }
	}
	attribute def Mass { attribute kg : Real; }
	attribute light : Mass { attribute :>> kg = 3.0; }
	variation attribute def Masses :> Mass {
		variant attribute lightCase = light;
	}
	part chosen : Case = Case::small;
	part holder {
		attribute viaDef : Real = Case::big.m;
		attribute viaValued : Real = Masses::lightCase.kg;
		attribute viaChosen : Real = chosen.m;
		part held : Body = Case::big;
		attribute viaHeld : Real = held.m;
	}
}
