// A connector inherited from a supertype names a part by its declared name; the subtype
// redefines the part under another name, so the end reaches the redefining part's port and,
// through the binding, the machine of the part nested inside it (SysML v2 §7.16, §8.3.9.5).
package RedefinedPartEnd {
	item def Go;
	item def Query;
	item def Ack;

	part def Peas {
		port toM1In;
		port toM1Out;
	}

	action def Handshake {
		in ref context : Peas;
		out ref item ack : Ack;
		first start then split;
		fork split;
		first split then ask;
		first split then heard;
		action ask {
			send new Query() via context.toM1Out;
		}
		action heard accept msg : Ack via context.toM1In;
		first ask then meet;
		first heard then meet;
		join meet;
		first meet then keep;
		action keep {
			assign ack := msg;
		}
		first keep then done;
	}

	part def Commander :> Peas {
		ref item lastAck : Ack[0..1];

		state def Life {
			entry; then idle;
			state idle;
			transition first idle accept Go then commanding;
			state commanding {
				do action run {
					first start then call;
					action call : Handshake;
					bind call.context = this;
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

	part def BoxSpec {
		port toM1In;
		port toM1Out;
	}

	part def Box :> BoxSpec {
		part peas : Commander;
		bind toM1In = peas.toM1In;
		bind peas.toM1Out = toM1Out;
	}

	part def M1CS {
		port cmdIn;
		port cmdOut;

		exhibit state life {
			entry; then idle;
			state idle;
			state installed;

			transition first idle
				accept Query via cmdIn
				do send new Ack() via cmdOut
				then installed;
		}
	}

	part def RigBase {
		part box : BoxSpec;
		part m1cs : M1CS;
		connect box.toM1Out to m1cs.cmdIn;
		connect box.toM1In to m1cs.cmdOut;
	}

	part def Rig :> RigBase {
		part logical : Box :>> box;
	}
}
