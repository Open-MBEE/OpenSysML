// A junction and a choice declared inside a composite state, on one compound
// transition into it: the composite is entered before the junction's segment
// runs its effect, and the choice's guards read what that effect wrote.
package Test {
	state Machine {
		attribute log : String = "";
		attribute x : Integer = 0;

		entry; then idle;
		state idle;
		state work {
			entry { assign log := log + "work(entry);"; }
			junction j;
			choice c;
			state w1 { entry { assign log := log + "w1(entry);"; } }
			state w2 { entry { assign log := log + "w2(entry);"; } }

			transition first j do { assign x := 1; assign log := log + "j(effect);"; } then c;
			transition first c if x == 1 do { assign log := log + "c1(effect);"; } then w1;
			transition first c do { assign log := log + "c2(effect);"; } then w2;
		}

		transition first idle accept Go do { assign log := log + "go(effect);"; } then j;
		transition first work accept Stop then done;
	}
	attribute def Go;
	attribute def Stop;
}
