package Cfg {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	part def Probe {
		attribute total : Real = 1.0;
		action shaky {
			attribute k : Integer = uniformInteger(0, 1);
			first start;
			then assign total := 10.0 / k;
			then done;
		}
		action steady { first start; then assign total := 4.0; then done; }
	}
	action def 'Group 0' { part target : Probe; perform action run ::> target.shaky; }
	action def 'Group 1' { part target : Probe; perform action run ::> target.steady; }
}
