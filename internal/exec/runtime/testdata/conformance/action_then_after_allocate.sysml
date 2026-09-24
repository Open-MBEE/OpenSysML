package test {
	private import ScalarValues::*;

	part def Controller;
	part engineController : Controller;

	// A positional `then` written after an allocation sequences from the action
	// node before it: the allocation relates members rather than declaring one.
	action driveTrain {
		first start;
		then action generateTorque {
			out engineTorque : Integer;
			assign engineTorque := 21;
		}
		allocate generateTorque to engineController;
		then action amplifyTorque {
			in torqueIn : Integer;
			out amplified : Integer;
			assign amplified := torqueIn * 2;
		}
		then done;

		flow from generateTorque.engineTorque to amplifyTorque.torqueIn;
	}
}
