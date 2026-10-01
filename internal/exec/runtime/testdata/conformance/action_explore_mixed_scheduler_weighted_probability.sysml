package test {
	private import ScalarValues::*;
	private import Stochastic::*;

	action mixed {
		attribute x : Integer = 0;
		attribute y : Integer = 0;
		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		then decide select;
		if x == 1 then draw;
		if x == 2 then keep;
		action draw {
			first start;
			then decide weighted;
			first weighted then one { @Probability { p = 0.3; } }
			first weighted then two { @Probability { p = 0.7; } }
			action one { assign y := 1; }
			then done;
			action two { assign y := 2; }
			then done;
		}
		action keep { assign y := 0; }
		then done;
		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
	}
}
