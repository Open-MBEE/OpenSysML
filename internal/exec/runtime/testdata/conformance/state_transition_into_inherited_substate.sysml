// A transition written in the outer machine targets a state inside the content
// a usage inherits.
package test {
	private import ScalarValues::*;

	state def Inner {
		attribute seen : Integer = 0;

		entry; then i1;
		state i1;
		state i2 {
			entry action mark {
				assign seen := seen + 1;
			}
		}
	}

	state def Outer {
		entry; then nested;
		state nested : Inner;
		transition first nested.i1 then nested.i2;
	}
}
