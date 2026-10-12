package test {
	private import ScalarValues::*;

	// Two same-named actions differ in the type of their one input. The winning
	// overload declares no input of its own: it inherits `label` from its general,
	// so the argument binds through the inherited parameter.
	action def Labelled {
		in label : String;
	}

	package Tags {
		action def tag :> Labelled {
			out code : Integer;
			first step;
			action step { assign code := 1; }
		}

		action def tag {
			in label : Integer;
			out code : Integer;
			first step;
			action step { assign code := 2; }
		}
	}

	action outer {
		private import Tags::*;
		out attribute chosen : Integer;

		first start;
		then action call = tag("x");
		then action fin { assign chosen := call.code; }
		then done;
	}
}
