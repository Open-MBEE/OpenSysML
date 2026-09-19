// Two connectors that join a sender to both names of one bound port — the
// assembly's boundary port and the inner port it is bound to — deliver one
// message, not one per name: the two ends are the same port.
package BindRelayAliasedEnds {
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
			accept Ping via i then Twice;

			state Twice {
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
		connect env.o to asm.child.i;
	}
}
