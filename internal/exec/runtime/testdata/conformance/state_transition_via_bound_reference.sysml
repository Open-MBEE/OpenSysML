// A transition's `via context.hears` names the port of whatever object the machine's
// reference parameter `context` is bound to, shadowing the performer's part of the
// same name as it does in any expression; `via this.context.hears` roots the path at
// the performer whatever the parameter holds. Each machine therefore fires on the
// acknowledgement crossing its own port and leaves the other pending.
package StateViaBoundReference {
	item def Go;
	item def Ack;

	port def AckOut {
		out item ack : Ack;
	}

	port def AckIn {
		in item ack : Ack;
	}

	part def Peer {
		port hears : AckIn;
	}

	part def Commander {
		part context : Peer;
		ref part partner : Peer;

		state def Life {
			in ref context : Peer;
			entry; then waiting;
			state waiting;
			transition first waiting accept Ack via context.hears then bound;
			state bound;
		}

		exhibit state life : Life { in :>> context = partner; }
	}

	part def Owner {
		part context : Peer;
		ref part partner : Peer;

		state def Watch {
			in ref context : Peer;
			entry; then waiting;
			state waiting;
			transition first waiting accept Ack via this.context.hears then own;
			state own;
		}

		exhibit state watch : Watch { in :>> context = partner; }
	}

	part def Sender {
		port toLive : AckOut;
		port toOwnerPart : AckOut;

		exhibit state life {
			entry; then idle;
			state idle;
			transition first idle accept Go then toLiveStep;
			state toLiveStep {
				entry send new Ack() via toLive;
			}
			transition first toLiveStep then toPartStep;
			state toPartStep {
				entry send new Ack() via toOwnerPart;
			}
			transition first toPartStep then sent;
			state sent;
		}
	}

	part def Rig {
		part commander : Commander {
			:>> partner = live;
		}
		part owner : Owner {
			:>> partner = live;
		}
		part live : Peer;
		part sender : Sender;
		connect sender.toLive to live.hears;
		connect sender.toOwnerPart to owner.context.hears;
	}
}
