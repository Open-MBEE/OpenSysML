// A guarded succession leads to the succession flow f: go holds, so b follows a
// and takes a's value along f.
package test {
	private import ScalarValues::*;

	action pass {
		attribute go : Boolean = true;
		first start then a;
		action a { out y : Integer; assign y := 1; }
		first a if go then f;
		succession flow f of Integer from a.y to b.v;
		action b { in v : Integer; }
	}
}
