// A node typed by an action def performs that action, and the action's own
// nested nodes become the node's subactions: a sibling reads the pin of a node
// inside the performed action through the typed node, `call.inner.v`, the same
// way it reads the node's own result pin, `call.result`. Here Seven's inner
// node computes 7, which the typed node exposes at both paths.
package test {
	private import ScalarValues::*;

	action def Seven {
		out result : Integer;

		first start;
		then action inner {
			out v : Integer;
			assign v := 3 + 4;
		}
		then action publish {
			assign result := inner.v;
		}
		then done;
	}

	action outer {
		out total : Integer = 0;
		out viaResult : Integer = 0;

		first start;
		then action call : Seven;
		then action read {
			assign total := call.inner.v;
			assign viaResult := call.result;
		}
		then done;
	}
}
