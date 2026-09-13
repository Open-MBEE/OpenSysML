// A dynamic target: a nested performance of a typed action writes through its
// output, which the reduction cannot see statically, so it depends on everything.
package test {
	private import ScalarValues::*;

	action def Add { in a : Integer; out r : Integer; first step; action step { assign r := a + 1; } }

	action dynamic {
		attribute x : Integer = 0;
		attribute y : Integer = 0;
		first start;
		fork split;
		action typed : Add { in a = x; }
		action writeX { assign x := 5; }
		action writeY { assign y := 1; }
		join sync;
		action collect { assign y := y + typed.r; }
		done;
		succession first start then split;
		succession first split then typed;
		succession first split then writeX;
		succession first split then writeY;
		succession first typed then sync;
		succession first writeX then sync;
		succession first writeY then sync;
		succession first sync then collect;
		succession first collect then done;
	}
}
