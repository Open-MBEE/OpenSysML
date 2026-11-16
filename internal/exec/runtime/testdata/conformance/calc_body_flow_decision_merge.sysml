// A calculation body stating a cycle through a merge and a decision: the flow
// repeats `step` until the guard fails, which no order of statements expresses.
package test {
	private import ScalarValues::*;

	calc counted {
		in n : Integer;
		attribute i : Integer := 0;
		attribute total : Integer := 0;
		first start then m;
		merge m;
		then action step {
			assign i := i + 1;
			then assign total := total + i;
		}
		then decide;
			if i < n then m;
			else done;
		return : Integer = total;
	}
}
