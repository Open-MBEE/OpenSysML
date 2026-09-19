// A require or assume member may name a sibling constraint or requirement by
// its bare name (`require massLimit;`): the member is that reference, so
// requiring it requires the referenced requirement's own conditions rather
// than a value of a feature named massLimit (SysML.xtext RequirementConstraintUsage).
package test {
	attribute mass = 120.0;

	requirement massLimit {
		require constraint { test::mass < 100.0 }
	}

	requirement vehicleOK {
		require massLimit;
	}
}
