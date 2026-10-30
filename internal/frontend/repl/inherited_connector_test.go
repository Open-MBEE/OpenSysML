package repl

import "testing"

// A connector declared in a general part def names its ends by the features it
// declared there. On an object of a specialization that redefines those parts,
// the names denote the redefining parts, so a send over the inherited connector
// reaches them — not a fresh object of the general part's type.
func TestSendOverInheritedConnectorReachesTheRedefiningPart(t *testing.T) {
	s := loadSource(t, `
package Gen {
	private import ScalarValues::*;
	port def P { in attribute v : Integer; }
	part def A { port o : ~P; }
	part def B { port i : P; }
	part def Top {
		part a : A;
		part b : B;
		connect a.o to b.i;
	}
}
package Beh {
	private import ScalarValues::*;
	private import Gen::*;
	item def Ping { attribute v : Integer; }
	item def Go;
	part def A2 :> A {
		exhibit state modes {
			entry; then idle;
			state idle;
			transition first idle accept Go do send new Ping(v = 7) via o then idle;
		}
	}
	part def B2 :> B {
		attribute got : Integer = 0;
		exhibit state modes {
			entry; then idle;
			state idle;
			transition first idle accept p : Ping via i do assign got := got + p.v then idle;
		}
	}
	part def Sys :> Top {
		part redefines a : A2;
		part redefines b : B2;
	}
	part sys : Sys;
}
`)
	wants(t, run(t, s, "%instantiate Beh::sys"), "✓ Created instance")
	run(t, s, "%state Beh::A2::modes sys.a")
	run(t, s, "%state Beh::B2::modes sys.b")
	wants(t, run(t, s, "%send Go() to sys.a"), "✓ Sent Go")
	run(t, s, "%advance 0.001")
	run(t, s, "%advance 0.001")
	wants(t, run(t, s, "%eval in sys.b : got"), "= 7")
	rejects(t, run(t, s, "%instances"), "Gen::Top::b")
}
