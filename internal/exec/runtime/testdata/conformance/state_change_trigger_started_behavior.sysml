package test {
	private import ScalarValues::*;

	attribute def Go;

	part def Pulse { attribute ready : Boolean = false; }

	part def Controller {
		part pulse : Pulse[0..1] {
			perform action pulsing {
				first start;
				then action rise { assign ready := true; }
				then action fall { assign ready := false; }
				then done;
			}
		}

		exhibit state monitor {
			entry; then outer;
			state outer {
				entry; then waiting;
				state waiting;
				state armed;
				transition first waiting accept Go
					do assign pulse := new Pulse()
					then armed;
			}
			state done;
			transition first outer accept when pulse != null and pulse#(1).ready then done;
		}
	}

	part controller : Controller;
}
