// An indexed binding end (an OpenSysML extension) reads one element of a
// collection-valued feature: `tank.levels#(2)` is the second level, 1-based as
// `#(` indexes everywhere, so the gauge bound to it reads 2.5 and nothing else,
// and a binding written the other way round selects the element as well.
package test {
	private import ScalarValues::*;

	part def Tank {
		attribute levels : Real[3] = (1.5, 2.5, 3.5);
	}

	part def Rig {
		part tank : Tank;
		attribute gauge : Real;
		attribute last : Real;
		bind gauge = tank.levels#(2);
		bind tank.levels#(3) = last;
	}

	part rig : Rig;
}
