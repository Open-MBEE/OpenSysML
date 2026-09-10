// The binding runs both ways: a send by the inner part through its own port
// leaves the assembly through the boundary port bound to it, and the
// context-level connector carries it to the environment's conjugated port.
package BindRelayOutbound {
	private import ScalarValues::*;

	attribute def Ping;

	port def PP {
		out item p : Ping;
	}

	part def Env {
		port o : ~PP;
		attribute got : Integer = 0;

		exhibit state sm {
			entry; then Idle;

			state Idle;
			accept Ping via o then Got;

			state Got {
				entry assign got := got + 1;
			}
		}
	}

	part def Inner {
		port i : PP;

		exhibit state sm {
			entry; then S1;

			state S1 {
				entry send new Ping() via i;
			}
		}
	}

	part def Assembly {
		port bo : PP;
		part child : Inner;

		bind bo = child.i;
	}

	part ctx {
		part env : Env;
		part asm : Assembly;

		connect asm.bo to env.o;
	}
}
