// A requirement-like body admits usage elements, so a connector written with
// its kind keyword (`connection connect …`) is a member there just as it is at
// package level (SysML RequirementBody → DefinitionBodyItem → UsageElement).
package P {
	part x;

	requirement r {
		connection connect r to x;
		flow x to x;
		message x to x;
	}

	constraint c {
		connection connect x to x;
	}

	concern cn {
		connection connect x to x;
	}

	verification def V {
		objective o {
			connection connect x to x;
		}
	}

	use case uc {
		connection connect x to x;
	}

	view v {
		connection connect x to x;
	}
}
