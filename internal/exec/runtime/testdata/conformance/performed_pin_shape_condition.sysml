// A bare Boolean pin holds a one-element sequence, which is its element (KerML §7.4.8):
// `if flag` and `while` over the pin branch as they do over a [1] pin.
package test {
	private import ScalarValues::*;

	action def Gate {
		in flag : Boolean;
		in limit : Integer[1];
		out seen : Integer[1] = 0;
		out count : Integer[1] = 0;
		first start;
		then action check {
			if flag { assign seen := 1; }
			while count < limit { assign count := count + 1; }
		}
		then done;
	}

	action probe {
		first start;
		then perform action g : Gate {
			in flag = true;
			in limit = 3;
		}
		then done;
	}
}
