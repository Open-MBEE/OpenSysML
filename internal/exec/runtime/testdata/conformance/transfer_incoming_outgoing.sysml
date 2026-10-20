package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	attribute def Ping;
	port def PP { out item p : Ping; }
	part def Sender {
		port o : PP;
		perform action snd {
			first start;
			then action s send new Ping() via o;
			then done;
		}
	}
	part def Receiver {
		port i : ~PP;
		attribute got : Integer = 0;
		perform action rcv {
			first start;
			then action r accept Ping via i;
			then action g assign got := got + 1;
			then done;
		}
	}
	part def Link {
		part x : Sender;
		part y : Receiver;
		connect x.o to y.i;
		attribute got : Integer = y.got;
		attribute sent : Integer = size(x.snd.outgoingTransfers);
		attribute sentByPart : Integer = size(x.outgoingTransfers);
		attribute sentByPort : Integer = size(x.o.outgoingTransfers);
		attribute received : Integer = size(y.rcv.incomingTransfers);
		attribute receivedByPort : Integer = size(y.i.incomingTransfers);
		attribute nothingIn : Integer = size(x.incomingTransfers);
		attribute sameTransfer : Boolean = x.snd.outgoingTransfers#(1) == y.rcv.incomingTransfers#(1);
		attribute fromSnd : Boolean = y.rcv.incomingTransfers#(1).source == x.snd;
		attribute toRcv : Boolean = y.rcv.incomingTransfers#(1).target == y.rcv;
		attribute carried : Integer = size(y.rcv.incomingTransfers#(1).payload);
	}
}
