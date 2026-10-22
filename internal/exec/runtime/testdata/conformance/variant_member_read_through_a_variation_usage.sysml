package test {
	private import ScalarValues::Real;
	part def Body { attribute m : Real; }
	variation part def Case :> Body {
		variant part big : Case { attribute :>> m = 2.0; }
		variant part small : Case { attribute :>> m = 1.0; }
	}
	part def Engine { attribute power : Real; }
	variation part engine : Engine {
		variant part electric : Engine { attribute :>> power = 100.0; }
		variant part diesel : Engine { attribute :>> power = 200.0; }
	}
	part cases : Case;
	part chosen : Case = Case::small;
	part holder {
		attribute viaTypedUsage : Real = cases.big.m;
		attribute viaChosenUsage : Real = chosen.big.m;
		attribute viaVariationUsage : Real = engine::electric.power;
		attribute viaVariationUsageChain : Real = engine.electric.power;
		attribute viaSelection : Real = chosen.m;
	}
}
