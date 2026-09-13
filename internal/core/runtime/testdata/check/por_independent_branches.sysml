// Independent branches: four branches each compute into a feature of their own
// over two nodes before meeting at the join, so every order reaches one outcome
// and the reduction explores few schedules of the many.
package test {
	private import ScalarValues::*;

	action parallel {
		attribute a : Integer = 0;
		attribute b : Integer = 0;
		attribute c : Integer = 0;
		attribute d : Integer = 0;
		first start;
		fork split;
		action wa { assign a := 1; }
		action wb { assign b := 2; }
		action wc { assign c := 3; }
		action wd { assign d := 4; }
		action da { assign a := a * 10; }
		action db { assign b := b * 10; }
		action dc { assign c := c * 10; }
		action dd { assign d := d * 10; }
		join sync;
		done;
		succession first start then split;
		succession first split then wa;
		succession first split then wb;
		succession first split then wc;
		succession first split then wd;
		succession first wa then da;
		succession first wb then db;
		succession first wc then dc;
		succession first wd then dd;
		succession first da then sync;
		succession first db then sync;
		succession first dc then sync;
		succession first dd then sync;
		succession first sync then done;
	}
}
