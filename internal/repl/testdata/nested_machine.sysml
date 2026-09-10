// A part whose nested part exhibits a machine, so the machine of an object reached
// through composition has to be addressed by a path or an identity. The entry
// action writes two slots, so a second performance of the machine is visible.
package Fleet {
	part def Rover {
		attribute level = 0;
		attribute log = "";

		exhibit state modes {
			entry; then waiting;
			state waiting {
				entry action w {
					assign log := log + "W";
					assign level := level + 10;
				}
			}
			accept after 5 [SI::s] then moving;
			state moving {
				entry action m {
					assign log := log + "M";
				}
			}
		}

		action bump {
			first b;
			action b {
				assign level := level + 1;
			}
		}
	}

	part def Driver {
		part r : Rover;
	}

	part rover : Rover;
	part driver : Driver;
}
