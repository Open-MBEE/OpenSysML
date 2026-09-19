package test {
	private import ScalarValues::*;

	// A binding at a node's pin whose other end is a feature chain writes the feature
	// of the object the chain reaches once the node completes, as an assignment
	// through the chain does: `bind add.sum = holder.value` fills `value` of the
	// part `holder` holds, and `holder.inner.mark` reaches through a nested part.
	part def Cell {
		attribute mark : Integer = 0;
	}

	part def Holder {
		attribute value : Integer = 0;
		part inner : Cell;
	}

	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;

		first step;
		action step { assign sum := a + b; }
	}

	action outer {
		part holder : Holder;
		out attribute total : Integer;
		out attribute deep : Integer;

		bind add.sum = holder.value;
		bind add2.a = holder.value;
		bind add2.sum = holder.inner.mark;

		first start;
		then action add : Adder {
			in a = 2;
			in b = 3;
		}
		then action add2 : Adder {
			in b = 10;
		}
		then action fin {
			assign total := holder.value;
			assign deep := holder.inner.mark;
		}
		then done;
	}
}
