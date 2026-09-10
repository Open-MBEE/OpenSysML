// A part holding several parts under one multi-valued feature, each exhibiting a
// machine: no path names one member, so a member is addressed by its identity.
package Depot {
	part def Rover {
		attribute level = 0;

		exhibit state modes {
			entry; then waiting;
			state waiting {
				entry action w {
					assign level := level + 10;
				}
			}
			accept after 5 [SI::s] then moving;
			state moving;
		}
	}

	part def Garage {
		part bays : Rover[2];
	}

	part garage : Garage;
}
