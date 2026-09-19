// A send through a parameter bound to an object leaves that object's port, follows its binding to
// the owner's boundary port and the connector on to the receiver (SysML v2 §7.16).
package SendDelegatedPort {
	item def Go;

	port def CmdPort {
		out item go : Go;
	}

	port def CmdIn {
		in item go : Go;
	}

	part def Peas {
		port toM1 : CmdPort;

		action def Command {
			in context : Peas;
			first start;
			then action go send new Go() via context.toM1;
			then done;
		}

		state def Life {
			entry; then commanding;
			state commanding {
				do action run {
					first start then call;
					action call : Command;
					bind call.context = this;
					first call then done;
				}
			}
		}

		exhibit state life : Life;
	}

	part def Box {
		port toM1 : CmdPort;
		part peas : Peas;
		bind toM1 = peas.toM1;
	}

	part def M1CS {
		port cmd : CmdIn;

		exhibit state life {
			entry; then idle;
			state idle;
			state installed;

			transition first idle
				accept Go via cmd
				then installed;
		}
	}

	part def Rig {
		part box : Box;
		part m1cs : M1CS;
		connect box.toM1 to m1cs.cmd;
	}
}
