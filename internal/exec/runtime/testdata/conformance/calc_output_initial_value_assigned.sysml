// An output declared with an initial value (`:=`) starts the body holding it;
// the body's assignments accumulate into it, and the calc yields what they left.
// A result parameter with an initial value is not a `return`: the body runs on.
package test {
	private import ScalarValues::*;
	calc def Total {
		in n : Integer;
		return total : Integer := 0;
		for i in 1..n {
			assign total := total + i;
		}
	}
	calc def Untouched {
		in n : Integer;
		out kept : Integer := n * 2;
	}
	calc probe {
		return : Integer = Total(4) + Untouched(5);
	}
}
