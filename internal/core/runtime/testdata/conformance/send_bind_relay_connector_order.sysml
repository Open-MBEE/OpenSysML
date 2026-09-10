// Delivery does not depend on declaration order: the connector is declared
// before the parts it joins and the sending part is declared last, and the
// message still crosses the binding to the inner part.
package BindRelayConnectorOrder {
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
		part child : Inner;
		bind bi = child.i;
		port bi : ~PP;
	}

	part ctx {
		connect env.o to asm.bi;
		part asm : Assembly;
		part env : Env;
	}
}
