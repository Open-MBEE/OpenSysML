// A directed use case usage in an action body is a parameter of the action, not a
// use case it performs: the action runs clean with Mission's subject unbound, and
// its one step is `work`.
package test {
	private import ScalarValues::*;

	part def Craft;

	use case def Mission {
		subject craft : Craft;
		action fly;
	}

	action def Config {
		out use case missions[*] : Mission;
		out count : Integer;
		action work { assign count := 1; }
	}
}
