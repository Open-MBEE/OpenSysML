// A send addresses the world the constraint judges — an effect outside the
// constraint's own performance, refused by a verdict.
package test {
	private import ScalarValues::*;

	part def Rig {
		attribute z : Real = 1;
		constraint sends { attribute y : Real = 1; send y to z; z > 0 }
	}
}
