// A transition into a choice declared inside a composite state enters the
// composite before the branch out of the choice runs its effect, as for a
// junction; the choice's guards are still read once the transition into it has
// run its effect, so the branch the incoming effect enables is the one taken.
package Test {
	state Machine {
		attribute log : String = "";
		attribute x : Integer = 0;

		entry; then idle;
		state idle;
		state work {
			entry { assign log := log + "work(entry);"; }
			exit { assign log := log + "work(exit);"; }
			choice pick;
			state w1 { entry { assign log := log + "w1(entry);"; } }
			state w2 { entry { assign log := log + "w2(entry);"; } }

			transition first pick if x == 1 do { assign log := log + "toW1(effect);"; } then w1;
			transition first pick do { assign log := log + "toW2(effect);"; } then w2;
		}

		transition first idle accept Go do { assign x := 1; assign log := log + "go(effect);"; } then pick;
		transition first work accept Stop then done;
	}
	attribute def Go;
	attribute def Stop;
}
