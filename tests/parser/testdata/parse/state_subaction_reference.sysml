package StateSubactionReferences {

	action def WarmUp;
	action def CoolDown;

	part controller {
		action warmUp : WarmUp;
		action monitor : WarmUp;
		action coolDown : CoolDown;

		state machine {
			state idle {
				entry warmUp;
				do monitor;
				exit coolDown;
			}

			state running {
				entry warmUp { in level = 1; }
				exit controller.coolDown;
			}
		}
	}
}
