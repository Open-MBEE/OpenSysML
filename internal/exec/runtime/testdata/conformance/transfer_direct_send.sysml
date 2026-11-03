package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	attribute def Ping;
	part def Receiver {
		attribute got : Integer = 0;
		perform action rcv {
			first start;
			then action r accept Ping;
			then action g assign got := got + 1;
			then done;
		}
	}
	part def Caller {
		part r : Receiver;
		perform action call {
			first start;
			then action s send new Ping() to r;
			then done;
		}
		attribute got : Integer = r.got;
		attribute sentByPerformance : Integer = size(call.outgoingTransfers);
		attribute sentByPart : Integer = size(outgoingTransfers);
		attribute fromCall : Boolean = r.rcv.incomingTransfers#(1).source == call;
	}
}
