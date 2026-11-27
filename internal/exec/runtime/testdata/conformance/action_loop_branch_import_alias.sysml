package Q { attribute def T; }
package test {
	private import ScalarValues::*;

	action run {
		attribute w : Integer = 0;
		attribute l : Integer = 0;
		attribute f : Integer = 0;
		attribute taken : Integer = 0;
		attribute skipped : Integer = 0;
		while w < 2 { private import Q::*; alias W for Q::T; assign w := w + 1; }
		loop { public import Q::*; alias L for Q::T; assign l := l + 1; } until l >= 3;
		for q in (1, 2, 3) { private import Q::*; alias F for Q::T; assign f := f + q; }
		if taken == 0 { private import Q::*; alias I for Q::T; assign taken := 1; }
		if skipped > 0 { assign skipped := 2; } else { private import Q::*; alias E for Q::T; assign skipped := 3; }
	}
}
