// A case performed as a step of an action body may itself wait on the clock
// (`accept after`): the performing action lists the case's wait among its own,
// so the clock advances to the instant and the case's later steps run there,
// then the action goes on.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import VerificationCases::*;

	part def Scenario {
		attribute log : Real default = 0.0;
		attribute arrived : Real default = -1.0;
	}

	verification def Scene {
		subject context : Scenario;
		action wait accept after 60.0 [s];
		then action mark {
			assign context.log := context.log + 1.0;
			assign context.arrived := localClock.currentTime;
		}
		return verdict : VerdictKind = VerdictKind::pass;
	}

	action def Config {
		part target : Scenario;
		out verdict : VerdictKind;
		out ended : Real;
		verification run : Scene { subject context = target; }
		then action after {
			assign verdict := run.verdict;
			assign ended := localClock.currentTime;
		}
	}
}
