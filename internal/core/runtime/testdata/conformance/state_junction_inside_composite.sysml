// A transition into a junction declared inside a composite state enters the
// composite on its way to the junction: the segment out of the junction is a
// performance enclosed by the composite, so its effect runs after the
// composite's entry behavior and before the target's.
package Test {
	state Machine {
		attribute log : String = "";
		attribute x : Integer = 0;

		entry; then idle;
		state idle;
		state work {
			entry { assign log := log + "work(entry);"; }
			exit { assign log := log + "work(exit);"; }
			junction split;
			state w1 { entry { assign log := log + "w1(entry);"; } }
			state w2 { entry { assign log := log + "w2(entry);"; } }

			transition first split if x == 1 do { assign log := log + "toW1(effect);"; } then w1;
			transition first split do { assign log := log + "toW2(effect);"; } then w2;
		}

		transition first idle accept Go do { assign log := log + "go(effect);"; } then split;
		transition first work accept Stop then done;
	}
	attribute def Go;
	attribute def Stop;
}
