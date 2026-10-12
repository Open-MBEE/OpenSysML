// A directed analysis or verification usage in an action body is a parameter of
// the action, not a case it performs: the action runs clean with neither Study
// nor Scene bound a subject, and its one step is `work`.
package test {
	private import ScalarValues::*;
	private import VerificationCases::*;

	part def Scenario {
		attribute log : Real default = 0.0;
	}

	analysis def Study {
		subject context : Scenario;
		return : Real;
		context.log
	}

	verification def Scene {
		subject context : Scenario;
		action mark { assign context.log := context.log + 1.0; }
		return verdict : VerdictKind = VerdictKind::pass;
	}

	action def Config {
		in analysis study : Study;
		out verification checks[*] : Scene;
		out count : Integer;
		action work { assign count := 1; }
	}
}
