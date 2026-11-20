// A node typed by an action performs it as one occurrence: the callee runs as the node's
// own object, so `run.call.inner.v` reads what the callee's performance of `inner` holds.
package test {
	private import ScalarValues::*;

	action def Callee {
		in a : Integer[1];
		out r : Integer[1];
		action inner { out v : Integer[1]; assign v := a * 10; }
		action last { assign r := inner.v + 1; }
		first start then inner;
		first inner then last;
		first last then done;
	}

	part plant {
		perform action run {
			out attribute total : Integer[1] = 0;
			action call : Callee { in a = 2; }
			action fin { assign total := call.r + call.inner.v; }
			first start then call;
			first call then fin;
			first fin then done;
		}
	}
}
