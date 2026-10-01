package test {
	private import ScalarValues::*;

	state def S {
		attribute total : Integer = 0;
		entry; then init;
		state init;
		state active {
			do action work {
				for i in 1..2 {
					action b { assign total := total + 10; }
					action a { assign total := total + 1; }
					succession a then b;
				}
			}
		}
		state done;
		succession first init then active;
		succession first active then done;
	}
}
