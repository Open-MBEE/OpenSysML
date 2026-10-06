// An untargeted assignment's implicit target is the constraint's own
// performance (SysML v2 §7.17.9): a name it holds no feature for — one of the
// constrained part's here — is refused, and the object left alone.
package test {
	private import ScalarValues::*;

	part def Rig {
		attribute z : Real = 1;
		constraint writesPart { attribute y : Real = 0; assign z := 5; z > 3 }
	}
}
