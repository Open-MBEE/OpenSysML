// Calculation bodies stating a token flow of their own (CalculationBodyItem : ActionBodyItem).
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
	calc forked {
		in n : Integer;
		attribute x : Integer := 0;
		attribute y : Integer := 0;
		first start then f;
		fork f;
			then left;
			then right;
		action left { assign x := n; }
		action right { assign y := n + 1; }
		join j;
		first left then j;
		first right then j;
		first j then done;
		return : Integer = x + y;
	}
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
	calc summed {
		in n : Integer;
		attribute i : Integer := n;
		attribute acc : Integer := 0;
		while i > 0 {
			action count { assign i := i - 1; }
			action take { assign acc := acc * 2 + i; }
			first take then count;
		}
		return : Integer = acc;
	}
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
