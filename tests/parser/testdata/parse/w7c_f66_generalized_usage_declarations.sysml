// A usage declaration may state its specializations without a name: the
// reference the notation subsets is followed by a FeatureSpecializationPart,
// whose multiplicity may come after the specializations
// (KerML.xtext FeatureSpecializationPart, SysML.xtext SatisfyRequirementUsage).
package W7CGeneralizedUsageDeclarations {
	part def Cylinder;
	part def LightRollBar;
	requirement def MassRequirement;
	attribute def MassValue;

	part def Vehicle {
		ref cylinderBR : Cylinder[*];
		ref rollBarBR : LightRollBar[*];
		ref lugBoltBR : Cylinder[*];
	}

	// `ref redefines x[n];` — the multiplicity follows the redefinition.
	part def 'Vehicle Model 1' :> Vehicle {
		ref redefines cylinderBR[4];
		ref redefines rollBarBR : LightRollBar[*];
		ref redefines lugBoltBR[24];
	}
	part def 'Vehicle Model 2' :> Vehicle {
		ref redefines cylinderBR[6..8];
		ref redefines rollBarBR[0];
	}

	// `verify r :>> req;` — the reference subsetted, then its redefinition.
	verification def MassTest {
		objective massVerificationObjective {
			verify requirement massRequirement : MassRequirement;
		}
	}
	requirement vehicleMassRequirement : MassRequirement;
	verification vehicleMassTest : MassTest {
		objective vehicleMassVerificationObjective {
			verify vehicleMassRequirement :>> massRequirement;
		}
	}

	// `variant use case` — a two-word kind keyword after a variant prefix.
	use case def UC;
	variation use case uc1 : UC {
		variant use case uc11;
		variant use case uc12;
	}

	// `assert not c { … }` — a negated assertion of a referenced constraint,
	// whose body redefines the constraint's parameters.
	part vehicle3 { attribute mass : MassValue; }
	part vehicle4 { attribute mass : MassValue; }
	constraint massLimitation { mass : MassValue; massLimit : MassValue; mass < massLimit }
	assert not massLimitation {
		:>> mass = vehicle3.mass;
		:>> massLimit = vehicle4.mass;
	}
	assert massLimitation;
}
