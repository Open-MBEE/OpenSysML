// Shared write: three branches assign one feature, and a fourth reads it. Every
// order of the writers is a distinct outcome; the reader's order matters too.
package test {
	private import ScalarValues::*;

	action race {
		attribute x : Integer = 0;
		attribute seen : Integer = -1;
		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		action c { assign x := 3; }
		action r { assign seen := x; }
		join sync;
		done;
		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first split then c;
		succession first split then r;
		succession first a then sync;
		succession first b then sync;
		succession first c then sync;
		succession first r then sync;
		succession first sync then done;
	}
}
