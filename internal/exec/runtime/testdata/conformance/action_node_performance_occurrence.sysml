// Each performance of an action node is an occurrence of its own under the performance
// around it (Actions.sysml `subactions :> subperformances`): `run.one.v` reads what it holds.
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
