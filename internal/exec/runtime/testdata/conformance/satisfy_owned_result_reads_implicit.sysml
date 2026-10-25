// A satisfy usage declared without `by` in a part definition takes the owning
// object as its subject (SysML v2 §7.21.4), so its `result` read through that
// object is the check on the object itself, and its subject reads that object.
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute verticalSpeed : Real;
	}

	requirement def Touchdown {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}

	part def Craft :> Lander {
		attribute :>> verticalSpeed = 0.5;
		satisfy requirement touchdown : Touchdown;
		attribute verdict : Boolean = touchdown.result;
		attribute subjectSpeed : Real = touchdown.lander.verticalSpeed;
	}
}
