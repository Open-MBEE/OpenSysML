// Two features holding one object: `twin` references the cell `cell` names, so
// the branches writing through either chain race for that one object's mark,
// and both names stand on the order, as does what is read from either.
package test {
	private import ScalarValues::*;
	part def Cell { attribute mark : Integer = 0; }
	action race {
		part cell : Cell;
		ref part twin : Cell = cell;
		attribute z : Integer = 0;
		first start;
		fork split;
		action a { assign cell.mark := 1; }
		action b { assign twin.mark := 2; }
		join sync;
		action derive { assign z := twin.mark; }
		done;
		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then derive;
		succession first derive then done;
	}
}
