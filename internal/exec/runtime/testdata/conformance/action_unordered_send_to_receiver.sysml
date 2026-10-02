package test {
	private import ScalarValues::*;

	attribute def Sig;
	attribute def SubSig :> Sig;

	part def Rx {
		attribute got : Integer := 0;
		exhibit state m {
			entry; then idle;
			state idle;
			transition first idle accept Sig then hit;
			state hit { entry assign got := 1; }
		}
	}

	// A send written directly in an action body is the action's one subaction: it
	// is performed, and the receiver's machine takes the transition it triggers.
	part def Sys {
		part rx : Rx;
		perform action drive { send new SubSig() to rx; }
	}
}
