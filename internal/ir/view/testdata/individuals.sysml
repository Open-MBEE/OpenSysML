package Probes {
	part def Probe;
	individual def Voyager :> Probe;
	occurrence def Flight;
	individual def Cruise :> Flight;

	// The individuals of a mission fold into the `individuals` compartment of its
	// box, whatever their keyword; the other usages into their keywords' own.
	occurrence def Mission {
		individual part voyager : Voyager;
		individual item relay : Voyager;
		part lander : Probe;
		individual occurrence cruise : Cruise;
	}
}

package IndividualViews {
	private import Views::*;

	view missionTree {
		expose Probes::Mission;
	}
}
