// Each pass of the while body performs the unordered `tick` three times:
// `anchor` advances the counter and `tick` accrues three per pass, so two
// passes leave c = 6.
package test {
	private import ScalarValues::*;

	action def U {
		attribute c : Integer = 0;
		attribute i : Integer = 0;
		first start then worker;
		action worker {
			while i < 2 {
				action anchor { assign i := i + 1; }
				first start then anchor;
				action tick[3] { assign c := c + 1; }
			}
		}
		then done;
	}
}
