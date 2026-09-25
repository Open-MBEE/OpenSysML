// Each transition into a join reads the payload its own accept names, whichever
// region's transition the dispatch draws first to fire the join.
package Test {
	private import ScalarValues::*;

	attribute def Level :> Integer;

	state Machine {
		attribute log : String = "";
		attribute left : Integer = 0;
		attribute right : Integer = 0;

		entry; then Work;
		state Work parallel {
			state a {
				entry; then a1;
				state a1;
				transition first a1 accept l : Level
					do action { assign left := l; assign log := log + "a "; }
					then sync;
			}
			state b {
				entry; then b1;
				state b1;
				transition first b1 accept r : Level
					do action { assign right := r + 1; assign log := log + "b "; }
					then sync;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
	}
}
