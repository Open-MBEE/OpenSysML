// Each performance of an action node is an occurrence of its own, a value of the
// node's feature on the performance around it (Performances.kerml `Performance :>
// Occurrence`; Actions.sysml `subactions :> subperformances`): read through the part
// performing `run`, `run.one.v` and `run.add.sum` are what the performances of `one`
// and `add` hold, and `add` holds the parameters of the action it performs.
package test {
	private import ScalarValues::*;

	action def Add {
		in a : Integer[1];
		in b : Integer[1];
		out sum : Integer[1];
		assign sum := a + b;
	}

	part plant {
		perform action run {
			out attribute total : Integer[1] = 0;
			action one { out v : Integer[1]; assign v := 1; }
			action add : Add { in a = one.v; in b = 5; }
			action fin { assign total := add.sum; }
			first start then one;
			first one then add;
			first add then fin;
			first fin then done;
		}
	}
}
