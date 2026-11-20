// A calculation body admits every action body item (CalculationBodyItem), so it
// may state a token flow of its own: the successions run `double` before `add`
// although `add` is declared first, and the result reads what the flow left.
package test {
	private import ScalarValues::*;

	calc sequenced {
		in n : Integer;
		attribute x : Integer := 1;
		action add { assign x := x + n; }
		action double { assign x := x * 2; }
		first start then double;
		first double then add;
		first add then done;
		return : Integer = x;
	}
}
