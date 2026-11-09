// A composite state's entry accept holds its regions back (SysML v2 §7.18.3,
// StatePerformances: the whole entry before the substates' entries): `working`
// is entered on Go and parks at its accept of Run; `left` and `right` start, and
// their states' entries read the log, only once Run has arrived and the entry
// has written 1 to it, so each region's slot carries that digit ahead of its own.
package StateEntryAcceptHoldsRegions {
	private import ScalarValues::*;

	item def Go;
	item def Run;
	item def Start;
	item def Ack;

	port def SenderPort {
		out item go : Go;
		out item run : Run;
		in item ack : Ack;
	}

	port def ReceiverPort {
		in item go : Go;
		in item run : Run;
		out item ack : Ack;
	}

	part def Receiver {
		port p : ReceiverPort;
		attribute log : Integer = 0;
		attribute leftLog : Integer = 0;
		attribute rightLog : Integer = 0;
		item ack : Ack;

		exhibit state life {
			entry; then idle;
			state idle;
			transition first idle accept Go via p then working;
			state working parallel {
				entry action {
					send ack via p;
					then accept Run via p;
					then assign log := log * 10 + 1;
				}
				state left {
					entry; then a;
					state a {
						entry assign leftLog := log * 10 + 2;
					}
				}
				state right {
					entry; then c;
					state c {
						entry assign rightLog := log * 10 + 3;
					}
				}
			}
		}
	}

	part def Sender {
		port out : SenderPort;
		item go : Go;
		item run : Run;

		exhibit state life {
			entry; then idle;
			state idle;
			state begin {
				entry send go via out;
			}
			state follow {
				entry send run via out;
			}
			state sent;
			transition first idle accept Start then begin;
			transition first begin accept Ack via out then follow;
			transition first follow then sent;
		}
	}

	part def Rig {
		part sender : Sender;
		part target : Receiver;
		connect sender.out to target.p;
	}
}
