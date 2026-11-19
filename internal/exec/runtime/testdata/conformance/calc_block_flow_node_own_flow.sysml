// An action declared in a loop body of a calculation states a flow of its own:
// it is performed as a node of the block's flow, and its succession runs `seed`
// before `grow` although `grow` is declared first.
package test {
	private import ScalarValues::*;

	calc grown {
		in n : Integer;
		attribute x : Integer := 0;
		for i in 1..2 {
			action round {
				action grow { assign x := x * n; }
				action seed { assign x := x + i; }
				first seed then grow;
			}
		}
		return : Integer = x;
	}
}
