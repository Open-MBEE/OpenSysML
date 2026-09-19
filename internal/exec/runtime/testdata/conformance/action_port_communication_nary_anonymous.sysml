package test {
	private import ScalarValues::*;

	action communicate {
		attribute atA : Integer = 0;
		attribute atB : Integer = 0;
		port outPort;
		port inA;
		port inB;
		connect (outPort, inA, inB);

		first start;
		action sender {
			send 100 via outPort;
		}
		action receiverA accept msgA : Integer via inA;
		action recordA {
			assign atA := msgA;
		}
		action receiverB accept msgB : Integer via inB;
		action recordB {
			assign atB := msgB;
		}
		done;
		succession first start then sender;
		succession first sender then receiverA;
		succession first receiverA then recordA;
		succession first recordA then receiverB;
		succession first receiverB then recordB;
		succession first recordB then done;
	}
}
