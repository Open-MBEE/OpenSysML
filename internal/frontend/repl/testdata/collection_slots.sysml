// A part with multi-valued features, whose contents are what inspecting it
// should show.
package Coll {
	part def Wheel {
		attribute radius = 0.3;
	}

	part def Rig {
		attribute mass = 100.0;
		attribute doubles[0..*] = mass * 2.0;
		part wheels : Wheel[2];
	}
}
