// Two succession flows leaving one output both follow its action, as a fork
// would: each successor runs once with the value.
package test {
	private import ScalarValues::*;

	action fan {
		first start then produce;
		action produce { out y : Real; assign y := 5.0; }
		action left { in v : Real; }
		action right { in v : Real; }
		succession flow of Real from produce.y to left.v;
		succession flow of Real from produce.y to right.v;
	}
}
