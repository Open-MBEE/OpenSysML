// A require or assume member may name the constraint it references through a
// feature chain (`require limits.massLimit;`): the member is that reference, so
// requiring it requires the referenced constraint's own conditions rather than
// a value read along the chain (SysML.xtext RequirementConstraintUsage,
// OwnedReferenceSubsetting).
package test {
	attribute mass = 120.0;

	part def Limits {
		constraint massLimit { test::mass < 100.0 }
	}
	part limits : Limits;

	requirement vehicleOK {
		require limits.massLimit;
	}
}
