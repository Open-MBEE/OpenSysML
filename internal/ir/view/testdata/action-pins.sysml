package Pins {
	private import ScalarValues::*;

	item def Bread;
	item def Toast;

	action def Heat {
		in b : Bread;
		out t : Toast;
	}

	action def Pack {
		in t : Toast;
		out boxed : Toast;
	}

	// An action's own parameters, bound to its nodes' pins by a nested pin's
	// value naming one and by an explicit bind.
	action def ToastBread {
		in bread : Bread;
		out toast : Toast;
		action heat : Heat { in b = bread; }
		action pack : Pack;
		bind pack.boxed = toast;
		flow heat.t to pack.t;
		first start; then heat; then pack; then done;
	}

	// The same wiring spelled by a qualified value, a nested pin's value naming
	// another node's pin, and a redefined output valued by the frame's.
	action def Wrap {
		in bread : Bread;
		out toast : Toast;
		action heat : Heat { in b = Wrap::bread; }
		action pack : Pack {
			in t = heat.t;
			out boxed :>> boxed = toast;
		}
		first start; then heat; then pack; then done;
	}

	// A usage takes its parameters from its type; a node of its own binds to one.
	action toaster : ToastBread {
		action heat2 : Heat { in b = bread; }
		first start; then heat2; then done;
	}

	// Two nodes' pins bound to each other, which is no flow.
	action def Chain {
		action heat : Heat;
		action pack : Pack;
		bind heat.t = pack.t;
		first start; then heat; then pack; then done;
	}

	// Bindings whose other end is no pin: a literal, an item of the action.
	action def Loose {
		item n : Toast;
		in bread : Bread;
		action heat : Heat { in b = bread; }
		action pack : Pack { in t = null; }
		bind pack.boxed = n;
		first start; then heat; then pack; then done;
	}

	// A node with a flow of its own binds its parameter to a node of that flow;
	// the binding is drawn on the node standing for it in the enclosing frame.
	action def Stack {
		in bread : Bread;
		action outer {
			in x : Bread = bread;
			action inner { in y : Bread = x; }
			first start; then inner; then done;
		}
		first start; then outer; then done;
	}

	// A node's own parameter sharing a frame parameter's name — declared by it
	// or inherited from its type — is what its pins' values name; no frame wire.
	action def Shadow {
		in x : Bread;
		in b : Bread;
		action child { in x : Bread; in y : Bread = x; }
		action heat : Heat { in c : Bread = b; }
		first start; then child; then heat; then done;
	}

	// A specializing action takes its frame pins from its general; a node of
	// its own binds to one.
	action def Specialized :> ToastBread {
		action heat2 : Heat { in b = bread; }
		first start; then heat2; then done;
	}

	action def Plain {
		action a;
		action b;
		first start; then a; then b; then done;
	}
}
