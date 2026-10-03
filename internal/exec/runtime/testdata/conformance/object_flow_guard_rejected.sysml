// A guarded succession leads to the succession flow f, so f moves a's value only
// when go holds. go is false: b, reached also by c's flow, runs once on c's value.
package test {
	private import ScalarValues::*;

	action pick {
		attribute go : Boolean = false;
		attribute runs : Integer = 0;
		attribute took : Integer = 0;
		first start then split;
		fork split;
		first split then a;
		first split then c;
		action a { out y : Integer; assign y := 1; }
		action c { out y : Integer; assign y := 2; }
		first a if go then f;
		succession flow f of Integer from a.y to b.v;
		succession flow of Integer from c.y to b.v;
		action b {
			in v : Integer;
			assign runs := runs + 1;
			assign took := v;
		}
	}
}
