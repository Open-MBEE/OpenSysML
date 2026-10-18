// Two branches out of a junction are enabled at once, and the state the
// transition leaves has an exit behavior: that exit runs whichever branch is
// drawn, so what it writes is fixed, and only what the branches themselves
// write is open to the draw.
package Test {
	state Machine {
		attribute route : Integer = 0;
		attribute log : Integer = 0;

		entry; then idle;
		state idle {
			exit assign log := 1;
		}
		junction split;
		state left;
		state right;

		transition first idle accept Go then split;
		transition first split do assign route := 1 then left;
		transition first split do assign route := 2 then right;
	}
}
