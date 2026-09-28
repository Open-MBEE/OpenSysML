package AcceptViaContextPortInStateDef {
	item def Go;
	item def Ping;
	item def Start;

	port def SenderPort {
		out item go : Go;
		out item ping : Ping;
	}

	port def ReceiverPort {
		in item go : Go;
		in item ping : Ping;
	}

	part def Receiver {
		port p : ReceiverPort;

		state def Life {
			in ref context : Receiver;
			entry; then ready;
			state ready;
			state waiting {
				do action buffer {
					first start then receive;
					action receive accept msg : Ping via context.p;
				}
			}
			state done;
			transition first ready accept Go via context.p then waiting;
			transition first waiting accept Go via context.p then done;
		}

		exhibit state life : Life { in ref :>> context = this; }
	}

	part def Sender {
		port out : SenderPort;
		item go : Go;
		item pingMessage : Ping;

		exhibit state life {
			entry; then idle;
			state idle;
			state begin {
				entry send go via out;
			}
			state ping {
				entry send pingMessage via out;
			}
			state finish {
				entry send go via out;
			}
			transition first idle accept Start then begin;
			transition first begin then ping;
			transition first ping then finish;
			transition first finish then sent;
			state sent;
		}
	}

	part def Rig {
		part sender : Sender;
		part target : Receiver;
		part decoy : Receiver;
		connect sender.out to target.p;
	}
}
