// A state machine a part exhibits is a performance occurrence: an entry action
// of one of its states is performed during it and during the sibling parts of
// the exhibiting object; a part destroyed there is not during anything after.
package test {
	private import ScalarValues::*;
	private import OccurrenceFunctions::*;

	part def Widget;

	part def Rover {
		part w : Widget;
		attribute machineDuring : Boolean;
		attribute partDuring : Boolean;
		attribute partAfter : Boolean;
		exhibit state modes {
			entry; then driving;
			state driving {
				entry action mark {
					assign machineDuring := isDuring(modes);
					assign partDuring := isDuring(w);
					assign w := destroy(w);
					assign partAfter := isDuring(w);
				}
			}
		}
	}
}
