package Loops {
	private import RequirementDerivation::*;
	private import Cycles::*;

	part def A {
		part b : B;
	}
	part def B {
		part a : A;
	}
	requirement def <'L1'> Looped;
	requirement primary : Looped;
	requirement secondary : Looped;

	#derivation connection {
		end #original ::> primary;
		end #derive ::> secondary;
	}
	#derivation connection {
		end #original ::> secondary;
		end #derive ::> primary;
	}
}

package Cycles {
	private import Loops::*;

	part def C;
}

package RobustViews {
	private import StandardViewDefinitions::*;

	view loopDefinitions : GeneralView {
		filter @SysML::Definition;
		expose Loops::*;
	}
	view loopRequirements : GeneralView {
		filter @SysML::RequirementUsage;
		expose Loops::*;
	}
	view importCycle : GeneralView {
		filter @SysML::Package;
		expose Loops;
		expose Cycles;
	}
	view noRequirements : GeneralView {
		filter @SysML::RequirementUsage;
		expose Cycles::C;
	}
	view unrecognized : GeneralView {
		filter @SysML::PartDefinition and @SysML::Definition;
		expose Loops::*;
	}
}
