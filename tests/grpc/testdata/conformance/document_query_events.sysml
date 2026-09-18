package Lamps {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	attribute def Toggle;
	state def LampMachine {
		entry; then off;
		state off;
		transition off_on first off accept Toggle then on;
		state on parallel {
			state light {
				entry; then run;
				state run;
			}
			state fan {
				entry; then slow;
				state slow;
			}
		}
	}
	part def Lamp { exhibit state lp : LampMachine; }
	part lamp : Lamp;

	calc def CurrentStates :> Query {
		in root : Element;
		Project(source = States(source = root), properties = ("machine", "statePath", "region"))
	}
	calc def Off :> Query {
		Project(source = InState(name = "off"), properties = ("qualifiedName"))
	}
	calc def Steps :> Query {
		in root : Element;
		Project(source = Events(source = root, kind = "entry"), properties = ("time", "state"))
	}
}
