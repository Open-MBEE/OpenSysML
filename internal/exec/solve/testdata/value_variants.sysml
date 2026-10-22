// A value-typed variation: every variant declares a value, so `==` on the point
// compares the values its variants declare rather than the choice itself, as
// KerML BaseFunctions::'==' is value equality for data values. Two variants
// declaring the same value are equal under it.
package test {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	part def Gear { attribute ratio : Integer; attribute scale : Real; attribute step : Integer; attribute divisor : Integer; }
	part gearFamily : Gear {
		variation attribute ratioChoice :>> ratio {
			variant attribute low : Integer = 3;
			variant attribute lowAgain : Integer = 3;
			variant attribute high : Integer = 5;
		}
		assert constraint isLow { ratioChoice == ratioChoice::low }
		assert constraint notHigh { ratioChoice != ratioChoice::high }
		assert constraint isThree { ratioChoice == 3 }
		assert constraint eachLow { ratioChoice->forAll { in ref r; r == ratioChoice::low } }
		assert constraint sameChoice { ratioChoice::low == ratioChoice::lowAgain }
		assert constraint pickedLow { ratioChoice::low == (if ratioChoice == ratioChoice::high ? ratioChoice::lowAgain else ratioChoice::low) }
		variation attribute scaleChoice :>> scale {
			variant attribute one : Integer = 1;
			variant attribute two : Integer = 2;
			variant attribute three : Real = 3.0;
		}
		assert constraint isTwo { scaleChoice == 2.0 }
		variation attribute stepChoice :>> step {
			variant attribute whole : Integer = 6 / divisor;
			variant attribute half : Integer = 3;
		}
		assert constraint halfStep { stepChoice == stepChoice::half }
	}
}
