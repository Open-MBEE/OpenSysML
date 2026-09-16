// A choice declared inside a composite state reads its guards once the composite
// is entered: the composite's entry behavior sets the value the guard tests, so
// the branch its entry enables is the one taken, for a transition into the choice
// and for a history's default transition into a choice of a nested composite.
package Test {
	state Machine {
		attribute log : String = "";
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		entry; then idle;
		state idle;
		state work {
			entry { assign x := 1; assign log := log + "work(entry);"; }
			exit { assign log := log + "work(exit);"; }
			choice pick;
			state w1 { entry { assign log := log + "w1(entry);"; } }
			state w2 { entry { assign log := log + "w2(entry);"; } }

			transition first pick if x == 1 do { assign log := log + "toW1(effect);"; } then w1;
			transition first pick do { assign log := log + "toW2(effect);"; } then w2;
		}
		state rest {
			entry { assign log := log + "rest(entry);"; }
			exit { assign log := log + "rest(exit);"; }
			history hist;
			state inner {
				entry { assign y := 1; assign log := log + "inner(entry);"; }
				choice which;
				state r1 { entry { assign log := log + "r1(entry);"; } }
				state r2 { entry { assign log := log + "r2(entry);"; } }

				transition first which if y == 1 do { assign log := log + "toR1(effect);"; } then r1;
				transition first which do { assign log := log + "toR2(effect);"; } then r2;
			}

			transition first hist do { assign log := log + "h(effect);"; } then inner.which;
		}

		transition first idle accept Go do { assign log := log + "go(effect);"; } then pick;
		transition first work accept Next do { assign log := log + "next(effect);"; } then rest.hist;
		transition first rest accept Stop then done;
	}
	attribute def Go;
	attribute def Next;
	attribute def Stop;
}
