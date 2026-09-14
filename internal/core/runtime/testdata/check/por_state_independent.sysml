// An action and a state machine on one clock touching features of their own:
// every interleaving of the action's steps with the machine's dispatches reaches
// one outcome, and the reduction explores few of the many.
package test {
	private import ScalarValues::*;

	action parallel {
		attribute a : Integer = 0;
		attribute b : Integer = 0;
		first start;
		fork split;
		action wa { assign a := 1; }
		action wb { assign b := 2; }
		action da { assign a := a * 10; }
		action db { assign b := b * 10; }
		join sync;
		done;
		succession first start then split;
		succession first split then wa;
		succession first split then wb;
		succession first wa then da;
		succession first wb then db;
		succession first da then sync;
		succession first db then sync;
		succession first sync then done;
	}

	state stepper {
		attribute steps : Integer = 0;
		entry; then one;
		state one;
		state two;
		state three;
		transition first one do assign steps := steps + 1 then two;
		transition first two do assign steps := steps + 1 then three;
		transition first three do assign steps := steps + 1 then done;
	}
}
