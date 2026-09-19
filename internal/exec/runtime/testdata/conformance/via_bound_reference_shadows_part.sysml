// The performer owns a part named `context` and the action binds a reference parameter of
// the same name to another object: `via context.toM1` follows the parameter, as any
// expression naming `context` would, so the query and its acknowledgement cross the
// partner's port, not the same-named part's (SysML v2 §7.16, §7.17).
package ViaBoundReferenceShadowsPart {
	item def Go;
	item def Query;
	item def Ack;

	port def QueryOut {
		out item query : Query;
		in item ack : Ack;
	}

	port def QueryIn {
		in item query : Query;
		out item ack : Ack;
	}

	part def Peer {
		port toM1 : QueryOut;
	}

	action def Handshake {
		in ref context : Peer;
		out ref item ack : Ack;
		first start then split;
		fork split;
		first split then ask;
		first split then heard;
		action ask {
			send new Query() via context.toM1;
		}
		action heard accept msg : Ack via context.toM1;
		first ask then meet;
		first heard then meet;
		join meet;
		first meet then keep;
		action keep {
			assign ack := msg;
		}
		first keep then done;
	}

	part def Commander {
		part context : Peer;
		ref part partner : Peer;
		ref item lastAck : Ack[0..1];

		state def Life {
			entry; then idle;
			state idle;
			transition first idle accept Go then commanding;
			state commanding {
				do action run {
					first start then call;
					action call : Handshake;
					bind call.context = partner;
					first call then keep;
					action keep {
						assign lastAck := call.ack;
					}
					first keep then done;
				}
			}
			state ready;
			transition first commanding then ready;
		}

		exhibit state life : Life;
	}

	part def M1CS {
		port cmd : QueryIn;

		exhibit state life {
			entry; then idle;
			state idle;
			state installed;

			transition first idle
				accept Query via cmd
				do send new Ack() via cmd
				then installed;
		}
	}

	part def Rig {
		part commander : Commander {
			:>> partner = live;
		}
		part live : Peer;
		part m1cs : M1CS;
		part decoy : M1CS;
		connect live.toM1 to m1cs.cmd;
		connect commander.context.toM1 to decoy.cmd;
	}
}
