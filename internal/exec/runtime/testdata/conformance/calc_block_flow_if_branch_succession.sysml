// A branch body of a calculation stating its own flow; the other branch is a
// plain statement body and keeps its statement order.
package test {
	private import ScalarValues::*;

	calc branched {
		in n : Integer;
		attribute x : Integer := 0;
		if n > 0 {
			action bump { assign x := x + 1; }
			action seed { assign x := n * 10; }
			first seed then bump;
		} else {
			assign x := 9;
		}
		return : Integer = x;
	}
}
