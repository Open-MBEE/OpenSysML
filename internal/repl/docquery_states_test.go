package repl

import "testing"

// stateQueryModel declares two lamps whose machine has orthogonal regions, with
// queries over the states the lamps are in and the steps their trace records.
const stateQueryModel = `package Lamps {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import SI::*;

	attribute def Toggle;
	attribute def Boost;
	attribute def Dim { attribute level : Integer; }

	state def LampMachine {
		attribute brightness : Integer = 0;
		entry; then off;
		state off;
		transition off_on first off accept Toggle then on;
		state on parallel {
			state light {
				entry; then run;
				state run;
				transition run_dim first run accept d : Dim do assign brightness := d.level then dim;
				state dim;
			}
			state fan {
				entry; then slow;
				state slow;
				transition slow_fast first slow accept Boost then fast;
				state fast;
			}
		}
		transition on_off first on accept Toggle then off;
	}
	part def Lamp { exhibit state lp : LampMachine; }
	part lamp1 : Lamp;
	part lamp2 : Lamp;

	calc def CurrentStates :> Query {
		Project(
			source = States(source = Objects(type = "Lamp")),
			properties = ("path", "machine", "statePath", "region", "enclosing")
		)
	}
	calc def Lit :> Query {
		InState(name = "on")
	}
	calc def Steps :> Query {
		in root : Element;
		Project(
			source = Events(source = root, kind = "accept, transition", since = 1 [s], before = 2.5 [s]),
			properties = ("time", "kind", "event", "from", "to", "payload")
		)
	}
}
`

func stateQuerySession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(stateQueryModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	return s
}

// TestRunQueryStatesOverSession checks %run-query reads the states the session's
// objects are in, one row per active leaf across regions, and the inverse lookup.
func TestRunQueryStatesOverSession(t *testing.T) {
	s := stateQuerySession(t)
	wants(t, run(t, s, "%run-query CurrentStates"), "✓ Query Lamps::CurrentStates returned 0 rows")

	run(t, s, "%instantiate lamp1")
	run(t, s, "%instantiate lamp2")
	wants(t, run(t, s, "%run-query CurrentStates"),
		"✓ Query Lamps::CurrentStates returned 2 rows",
		"Row 1: Lamps::lamp1.lp in off",
		`statePath = "off"`,
		"Row 2: Lamps::lamp2.lp in off")
	wants(t, run(t, s, "%run-query Lit"), "✓ Query Lamps::Lit returned 0 rows")

	run(t, s, "%state lp lamp1")
	run(t, s, "%send Toggle")
	run(t, s, "%advance 1")
	wants(t, run(t, s, "%run-query CurrentStates"),
		"✓ Query Lamps::CurrentStates returned 3 rows",
		"Row 1: Lamps::lamp1.lp in on.run",
		`region = "light"`,
		`enclosing = "on"`,
		"Row 2: Lamps::lamp1.lp in on.slow",
		`region = "fan"`,
		"Row 3: Lamps::lamp2.lp in off")
	wants(t, run(t, s, "%run-query Lit"),
		"✓ Query Lamps::Lit returned 1 row",
		"Row 1: Lamps::lamp1 (#1)")
}

// TestRunQueryEventsOverSession checks %run-query reads the recorded trace: refused
// without tracing, then the interval's accepts and transitions, refused again after %trace off.
func TestRunQueryEventsOverSession(t *testing.T) {
	s := stateQuerySession(t)
	run(t, s, "%instantiate lamp1")
	run(t, s, "%instantiate lamp2")
	wants(t, run(t, s, "%run-query Steps root=lamp1"),
		"error:", "reads the session's trace, and this session records none")

	run(t, s, "%trace on")
	run(t, s, "%state lp lamp1")
	run(t, s, "%send Toggle")
	run(t, s, "%advance 1")
	run(t, s, "%send Dim(level=3)")
	run(t, s, "%advance 1")
	run(t, s, "%send Boost")
	run(t, s, "%send Toggle to lamp2")
	run(t, s, "%advance 0.5")
	run(t, s, "%send Toggle to lamp2")
	run(t, s, "%advance 0.5")

	wants(t, run(t, s, "%run-query Steps root=lamp1"),
		"✓ Query Lamps::Steps returned 4 rows",
		"Columns: time, kind, event, from, to, payload",
		"Row 1: t=1 Lamps::lamp1.lp: accept Dim",
		"time = 1.0 [s]",
		`kind = "accept"`,
		`payload = "level = 3"`,
		"Row 2: t=1 Lamps::lamp1.lp: transition: run -> dim (event: accept Dim)",
		`from = "run"`,
		`to = "dim"`,
		"Row 3: t=2 Lamps::lamp1.lp: accept Boost",
		"Row 4: t=2 Lamps::lamp1.lp: transition: slow -> fast (event: accept Boost)")
	wants(t, run(t, s, "%run-query Steps root=lamp2"),
		"✓ Query Lamps::Steps returned 2 rows",
		"Row 1: t=2 Lamps::lamp2.lp: accept Toggle",
		"Row 2: t=2 Lamps::lamp2.lp: transition: off -> on (event: accept Toggle)")

	run(t, s, "%trace off")
	wants(t, run(t, s, "%run-query Steps root=lamp1"),
		"error:", "reads the session's trace, and this session records none")
}
