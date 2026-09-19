// A change trigger fires on its condition rising: a self-transition on a
// condition that stays true takes its edge once rather than forever, so the run
// suspends instead of exhausting the event budget.
package test {
	state Machine {
		attribute log : Integer = 0;
		attribute ready : Boolean = true;

		entry; then start;
		state start;
		state waiting;

		succession first start then waiting;
		transition first waiting accept when ready do assign log := log * 10 + 1 then waiting;
	}
}
