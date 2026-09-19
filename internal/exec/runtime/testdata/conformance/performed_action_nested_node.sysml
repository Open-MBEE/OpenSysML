// A nested action node writes the root action performance occurrence rather than
// an occurrence of its own: the node performing an inherited body returns its
// output into the features the performed action holds.
package test {
	private import ScalarValues::*;

	action def Step {
		out attribute distance : Integer;

		action move {
			assign distance := 21;
		}
		first move;
	}

	action def Travel {
		attribute distance : Integer = 0;

		action leg : Step;
		first leg;
	}

	part def Rover {
		perform action trip : Travel;
	}
}
