// A branch body of a calculation binding a pin of its action node: the node is
// performed and reads the bound value through its input.
package test {
	private import ScalarValues::*;

	calc bound {
		in n : Integer;
		attribute seen : Integer := 0;
		if n > 0 {
			action take { in w : Integer; assign seen := w + 1; }
			bind take.w = n;
		}
		return : Integer = seen;
	}
}
