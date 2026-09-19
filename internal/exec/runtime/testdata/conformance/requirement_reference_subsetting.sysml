// A require member may name a requirement by qualified reference instead of
// stating a condition, with `;` for its body: requiring it requires that
// requirement's own conditions (SysML.xtext RequirementConstraintUsage).
package test {
	attribute mass = 120.0;

	package Limits {
		requirement massLimit {
			require constraint { test::mass < 100.0 }
		}
	}

	requirement vehicleOK {
		require Limits::massLimit;
	}
}
