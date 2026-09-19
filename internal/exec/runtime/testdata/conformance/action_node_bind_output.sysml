package test {
	private import ScalarValues::*;

	// A binding connector at a node's output pin carries what the node produced to
	// the feature at the other end once the node completes: an enclosing
	// attribute, or the input pin of a node downstream.
	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;

		first step;
		action step { assign sum := a + b; }
	}

	action outer {
		out attribute total : Integer;
		out attribute doubled : Integer;

		bind total = add.sum;
		bind add.sum = twice.n;

		first start;
		then action add : Adder {
			in a = 2;
			in b = 3;
		}
		then action twice { in n : Integer; assign doubled := n * 2; }
		then done;
	}
}
