// One occurrence selects a transition in each region of a parallel state; the
// two firings are drawn one unit at a time — each region's source exit, then
// its effect — so a source's exit may fall between the other firing's exit and
// effect. The six linearizations of two chains of two are the admissible set.
// The targets then complete into a join, whose incoming segments are drawn
// whole, one at a time.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then work;
		state work parallel {
			state left {
				entry; then l1;
				state l1 { exit { assign log := log + "l1(exit) "; } }
				state l2;
				transition first l1 accept Go do { assign log := log + "T1(effect) "; } then l2;
			}
			state right {
				entry; then r1;
				state r1 { exit { assign log := log + "r1(exit) "; } }
				state r2;
				transition first r1 accept Go do { assign log := log + "T2(effect) "; } then r2;
			}
		}
		join sync;

		transition first l2 then sync;
		transition first r2 then sync;
		transition first sync do { assign log := log + "T3(effect) "; } then done;
	}
	attribute def Go;
}
