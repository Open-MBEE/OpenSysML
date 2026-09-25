// An action's declared attribute belongs to the action performance the perform
// usage holds, not to a like-named attribute of the object performing it.
package test {
	private import ScalarValues::*;

	action def Bump {
		attribute scratch : Integer = 0;

		action step {
			assign scratch := 7;
		}
		first step;
	}

	part def Host {
		attribute scratch : Integer = 10;
		perform action work : Bump;
	}
}
