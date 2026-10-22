// A value-typed variation: every variant declares a value, so `==` on the point
// compares the values its variants declare rather than the choice itself, as
// KerML BaseFunctions::'==' is value equality for data values. Two variants
// declaring the same value are equal under it.
package test {
	private import ScalarValues::Integer;
	private import ControlFunctions::*;
	part def Gear { attribute ratio : Integer; }
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
	}
}
