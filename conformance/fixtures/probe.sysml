package Cfg {
	private import ScalarValues::*;
	part def Probe {
		attribute total : Real = 1.0;
		action steady { first start; then assign total := 4.0; then done; }
	}
	action def 'Group 1' { part target : Probe; perform action run ::> target.steady; }
}
