// An accept in a branch the entry body does not take holds nothing: `deciding`'s
// entry runs to its end at the instant of entry (`armed` is false), so its entry
// transitions read the `n` it assigned and choose `ready`, and no message is waited for.
package StateEntryAcceptUntakenBranch {
	private import ScalarValues::*;

	item def Go;
	item def Start;

	port def SenderPort {
		out item go : Go;
	}

	port def ReceiverPort {
		in item go : Go;
	}

	part def Receiver {
		port p : ReceiverPort;
		attribute armed : Boolean = false;
		attribute n : Integer = 0;

		exhibit state life {
			entry; then idle;
			state idle;
			transition first idle accept Go via p then deciding;
			state deciding {
				entry action {
					if armed {
						accept Go via p;
					}
					assign n := 1;
				}
				if n == 1 then ready;
				if n != 1 then wrong;
				state ready;
				state wrong;
			}
		}
	}

	part def Sender {
		port out : SenderPort;
		item go : Go;

		exhibit state life {
			entry; then idle;
			state idle;
			state begin {
				entry send go via out;
			}
			state sent;
			transition first idle accept Start then begin;
			transition first begin then sent;
		}
	}

	part def Rig {
		part sender : Sender;
		part target : Receiver;
		connect sender.out to target.p;
	}
}
