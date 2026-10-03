// Two succession flows queue a's value twice at b.v; b's first run takes one. c later
// reaches b by control alone, after a has retired, and b runs on the value still queued.
package test {
	private import ScalarValues::*;

	action relay {
		attribute runs : Integer = 0;
		attribute took : Integer = 0;
		first start then a;
		action a { out y : Integer; assign y := 7; }
		succession flow f1 of Integer from a.y to b.v;
		succession flow f2 of Integer from a.y to b.v;
		action b {
			in v : Integer [1];
			assign runs := runs + 1;
			assign took := v;
		}
		first b then check;
		decide check;
		if runs < 2 then c;
		else done;
		action c;
		first c then b;
	}
}
