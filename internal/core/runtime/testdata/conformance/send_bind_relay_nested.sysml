// Bindings chain through several levels of assembly: the outer assembly binds
// its boundary port to the inner assembly's, which binds its own to the leaf
// part's, and a message delivered at the outermost port reaches the leaf.
package BindRelayNested {
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

	part def Leaf {
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

	part def InnerAssembly {
		port bi : ~PP;
		part leaf : Leaf;

		bind bi = leaf.i;
	}

	part def OuterAssembly {
		port bi : ~PP;
		part inner : InnerAssembly;

		bind bi = inner.bi;
	}

	part ctx {
		part env : Env;
		part asm : OuterAssembly;

		connect env.o to asm.bi;
	}
}
