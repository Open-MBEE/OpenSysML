// Read after the incoming effect, the choice's first branch holds; the second's
// guard has no result (it divides by zero). A guard with no result is not true,
// so that branch is not taken and the run goes on by the first; the executor
// reports the guard it could not evaluate.
package Test {
	state Router {
		attribute level : Integer = 0;
		attribute route : Integer = 0;

		entry; then idle;
		state idle;
		choice pick;
		state low { entry { assign route := 1; } }
		state high { entry { assign route := 2; } }

		transition first idle accept Go do assign level := 8 then pick;
		transition first pick if level > 5 then low;
		transition first pick if 1 / (level - 8) > 0 then high;
	}
}
