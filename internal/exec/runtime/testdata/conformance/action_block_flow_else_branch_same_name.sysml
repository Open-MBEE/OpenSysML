// Each branch of an `if` declares its own node `p`, and a read of `p.v` in a
// branch is of that branch's node, not of the first one so named: here the
// else branch runs, its `p` computes 30, and the assignment after it reads 30.
// The then branch's `p` never performs, and is not reported.
package test {
	private import ScalarValues::*;

	action run {
		attribute value : Integer = 1;
		attribute total : Integer = 0;

		first start;
		then action choose {
			if value > 3 {
				action p {
					out v : Integer;
					assign v := value * 2;
				}
				assign total := p.v;
			} else {
				action p {
					out v : Integer;
					assign v := value * 30;
				}
				assign total := p.v;
			}
		}
		then done;
	}
}
