package test {
	private import ScalarValues::*;

	// A false guard on one of two successions out of an ordinary node leaves out
	// only that HappensBefore link (transitionLink : HappensBefore[0..1]): c is
	// performed, b is not. d, which both branches reach, awaits only what can still
	// reach it, as behind a fork branch a guard pruned, so it performs once after c.
	action pruned {
		attribute armed : Integer = 0;
		attribute bRan : Boolean = false;
		attribute cRan : Boolean = false;
		attribute hits : Integer = 0;

		first start;
		then action a;
		action b { assign bRan := true; }
		action c { assign cRan := true; }
		action d { assign hits := hits + 1; }

		succession first a if armed > 0 then b;
		succession first a then c;
		succession first b then d;
		succession first c then d;
		succession first d then done;
	}
}
