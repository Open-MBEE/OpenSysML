// The then body taken is one performance, performing `a` twice; the else body is
// not performed, and `none[0]` in the then body performs nothing.
package test {
	private import ScalarValues::*;

	action def IfRep {
		attribute c : Integer = 0;
		attribute flag : Boolean = true;
		first start then worker;
		action worker {
			if flag {
				first start then a;
				action a[2] { assign c := c + 5; }
				action none[0] { assign c := c + 100; }
			} else {
				first start then b;
				action b[3] { assign c := c + 1000; }
			}
		}
		then done;
	}
}
