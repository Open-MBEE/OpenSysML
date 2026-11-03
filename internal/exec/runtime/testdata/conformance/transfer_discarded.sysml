package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	attribute def Ping;
	attribute def Pong;
	part def Deaf {
		exhibit state m {
			entry; then idle;
			state idle;
			transition idle_done first idle accept Pong then over;
			state over;
		}
	}
	part def Caller {
		part d : Deaf;
		perform action call {
			first start;
			then action s send new Ping() to d;
			then done;
		}
		attribute sent : Integer = size(call.outgoingTransfers);
		attribute landed : Integer = size(d.incomingTransfers);
		attribute ended : Boolean = call.outgoingTransfers#(1).endShot == call.outgoingTransfers#(1).endShot;
	}
}
