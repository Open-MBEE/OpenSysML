// Quantifiers over a variation point: a configuration holds only the variant
// the point selects, so `->forAll` and `->exists` over it compare the selection
// with a variant. A point that may hold no object, or a body comparing the
// iterate with something other than a variant, stays outside the subset.
package test {
	private import ControlFunctions::*;
	alias each for ControlFunctions::forAll;
	part def Wheel;
	part def NarrowWheel :> Wheel;
	part def WideWheel :> Wheel;
	part def Engine;

	part def Car {
		part engine : Engine;
		part wheels : Wheel[2];
		part spare : Wheel[0..1];
	}

	part carFamily : Car {
		variation part engineChoice :>> engine {
			variant part small : Engine;
			variant part big : Engine;
		}
		variation part wheelChoice :>> wheels {
			variant part narrow : NarrowWheel;
			variant part wide : WideWheel;
		}
		variation part spareChoice :>> spare {
			variant part narrowSpare : NarrowWheel;
			variant part wideSpare : WideWheel;
		}

		assert constraint allNarrow {
			engineChoice == engineChoice::small implies wheelChoice->forAll { in ref w; w == wheelChoice::narrow }
		}
		assert constraint someWide {
			engineChoice == engineChoice::big implies wheelChoice->exists { in ref w; w == wheelChoice::wide }
		}
		assert constraint noneWide {
			engineChoice == engineChoice::small implies wheelChoice->forAll { in ref w; wheelChoice::wide != w }
		}
		assert constraint aliasedAll {
			wheelChoice->each { in ref w; w == wheelChoice::narrow }
		}
		assert constraint spareNarrow {
			spareChoice->forAll { in ref w; w == spareChoice::narrowSpare }
		}
		assert constraint wheelIsEngine {
			wheelChoice->forAll { in ref w; w == engineChoice::small }
		}
		assert constraint twoWheels {
			wheelChoice->size() == 2
		}
		assert constraint ownForAll {
			wheelChoice->Own::forAll { in ref w; w == wheelChoice::narrow }
		}
	}

	package Own {
		calc def forAll { in ref seq : Anything[0..*]; in expr; return : Boolean = false; }
	}

	part def Fleet { in attribute n : Natural; part cars : Car[n..3]; }
	part fleet : Fleet {
		variation part carChoice :>> cars { variant part sedan : Car; variant part wagon : Car; }
		assert constraint unknownBound {
			carChoice->forAll { in ref c; c == carChoice::sedan }
		}
	}
}
