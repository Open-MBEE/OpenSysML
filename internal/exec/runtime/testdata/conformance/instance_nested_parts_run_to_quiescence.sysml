// An object whose nested parts exchange a signal at start-up: the sender's machine
// sends on entry, the receiver's counts what it accepts, and the whole runs to
// quiescence when the object is created, so a read of the receiver's count — and a
// default of the object that reads it — sees the effect of the exchange.
package P {
	private import ScalarValues::*;
	attribute def Ping;
	port def PP { out item p : Ping; }
	part def Sender {
		port o : PP;
		exhibit state sm { entry; then S1; state S1 { entry send new Ping() via o; } }
	}
	part def Recv {
		port i : ~PP;
		attribute got : Integer = 0;
		exhibit state sm {
			entry; then Idle;
			state Idle;
			accept Ping via i then Got;
			state Got { entry assign got := got + 1; }
		}
	}
	part ctx {
		part s : Sender;
		part recv : Recv;
		connect s.o to recv.i;
		attribute received : Integer = recv.got;
	}
}
