// A chained assignment target writes a feature of the object the chain reaches
// — an effect outside the constraint's own performance, refused by a verdict.
package test {
	private import ScalarValues::*;

	part def Rig {
		attribute z : Real = 1;
		constraint writesChain { attribute v : Rig; assign v.z := 5; v.z > 3 }
	}
}
