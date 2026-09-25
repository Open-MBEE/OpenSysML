// A part whose attributes state values the model already fixes — a number, an
// enumeration literal, a quantity in a unit that is not the base one, and a
// string — with constraints reading them, so a query can fix what the model
// states and leave the rest free.
package test {
	private import ScalarValues::*;
	public import SI::*;
	public import ISQ::*;

	enum def Finish {
		enum polished;
		enum brushed;
	}

	part def Panel {
		attribute width : Integer = 4;
		attribute height : Integer;
		attribute finish : Finish = Finish::polished;
		attribute maxSpeed : ISQSpaceTime::SpeedValue = 5.4 [km/h];
		attribute clearance : ISQBase::LengthValue = 2.0 [m];
		attribute label : String = "left";

		assert constraint fits {
			width + height <= 10
		}

		assert constraint polishedIsWide {
			finish == Finish::polished implies width >= 6
		}

		assert constraint speedIsBounded {
			maxSpeed <= 2.0 [m/s]
		}

		assert constraint labelled {
			label == "left"
		}
	}
}
