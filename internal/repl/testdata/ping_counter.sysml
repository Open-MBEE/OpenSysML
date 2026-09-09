// A part whose nested parts exchange a signal at start-up: the sender's machine
// sends on entry and the receiver's counts what it accepts, so a read of the count
// after %instantiate tells whether it sees the object that ran or a fresh copy.
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
	}
}
