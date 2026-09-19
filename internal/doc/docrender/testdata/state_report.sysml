package Lamps {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import SI::*;

	attribute def Toggle;
	attribute def Boost;
	attribute def Dim { attribute level : Integer; }
	attribute def Report;

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
				state fast {
					entry send new Report() to panel;
				}
			}
		}
		transition on_off first on accept Toggle then off;
	}
	part def Lamp { exhibit state lp : LampMachine; }
	part def Panel;
	part lamp1 : Lamp;
	part lamp2 : Lamp;
	part panel : Panel;

	calc def CurrentStates :> Query {
		Project(
			source = States(source = Objects(type = "Lamp")),
			properties = ("path", "machine", "statePath", "region", "enclosing")
		)
	}
	calc def Lit :> Query {
		Project(source = InState(name = "on"), properties = ("qualifiedName"))
	}
	calc def Steps :> Query {
		in root : Element;
		Project(
			source = Events(source = root, kind = "accept, transition, send", since = 1 [s], before = 2.5 [s]),
			properties = ("time", "kind", "event", "from", "to", "target", "payload")
		)
	}
	calc def Entered :> Query {
		WhereFeature(source = Events(kind = "entry"), 'feature' = "state", operator = "=", value = "on")
	}
	calc def Standing :> Query {
		States(source = Objects(type = "Lamp"))
	}

	part def LampReport :> Document {
		attribute redefines title = "Lamp Report";

		part intro : Paragraph {
			attribute redefines text = "Where the lamps stand and what they did.";
		}
		part states : Table {
			attribute redefines caption = "Active states of every lamp";
			calc rows : CurrentStates;
		}
		part lit : Table {
			attribute redefines caption = "Lamps that are on";
			calc rows : Lit;
		}
		part steps : Table {
			attribute redefines caption = "What lamp1 did from 1 s up to 2.5 s";
			calc rows : Steps {
				in root = lamp1;
			}
		}
		part entered : List {
			calc items : Entered;
		}
		part standing : List {
			calc items : Standing;
		}
	}
}
