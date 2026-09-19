// A binding connector between an assembly's boundary port and the port of a
// part it holds makes them one port: a message a context-level connector
// delivers to the boundary port arrives at the inner part, whose machine
// accepts it on its own port.
package BindRelayInbound {
	private import ScalarValues::*;

	attribute def Ping;

	port def PP {
		out item p : Ping;
	}

	part def Env {
		port o : PP;

		exhibit state sm {
			entry; then S1;

			state S1 {
				entry send new Ping() via o;
			}
		}
	}

	part def Inner {
		port i : ~PP;
		attribute got : Integer = 0;

		exhibit state sm {
			entry; then Idle;

			state Idle;
			accept Ping via i then Got;

			state Got {
				entry assign got := got + 1;
			}
		}
	}

	part def Assembly {
		port bi : ~PP;
		part child : Inner;

		bind bi = child.i;
	}

	part ctx {
		part env : Env;
		part asm : Assembly;

		connect env.o to asm.bi;
	}
}
