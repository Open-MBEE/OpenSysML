// Untyped ports are Ports::Port objects, so a binding joins the box's boundary port and the nested
// part's port as one: a send arriving at the box reaches the machine accepting via the nested port.
package SendUntypedPort {
	item def Go;

	part def Sender {
		port tx;

		action def Fire {
			first start;
			then action go send new Go() via this.tx;
			then done;
		}

		perform action fire : Fire;
	}

	part def Receiver {
		port rx;

		exhibit state life {
			entry; then idle;
			state idle;
			state got;

			transition first idle
				accept Go via rx
				then got;
		}
	}

	part def Box {
		port cmd;
		part r : Receiver;
		bind cmd = r.rx;
	}

	part def Rig {
		part s : Sender;
		part b : Box;
		connect s.tx to b.cmd;
	}
}
