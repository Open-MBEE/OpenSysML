package test {
	metadata def Safety;

	package Lib {
		#Safety part def Belt;
		part def Radio;
	}

	// The view exposes only the annotated element, which is the classification a
	// filter decides at name resolution and this constraint decides at runtime.
	view safetyView {
		expose Lib::*[@Safety];
	}

	constraint exposedAreClassified {
		Lib::Belt @ Safety and not (Lib::Radio @ Safety)
			and safetyView @@ SysML::ViewUsage and not (safetyView @@ SysML::PartDefinition)
	}
}
