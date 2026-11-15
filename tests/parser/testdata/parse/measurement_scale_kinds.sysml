// Each kind of measurement scale gives its values what its definition states.
// An OrdinalScale orders its values and nothing more. A CyclicRatioScale is a
// ratio scale: its values are magnitudes in its unit, equivalent over its
// modulus. A LogarithmicScale maps a quantity x to factor * log_base((x /
// referenceQuantity) ** exponent), so its values convert and compare through
// that mapping, and have no interval between them.
package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::*;
	private import QuantityCalculations::*;

	attribute mohs : OrdinalScale {
		:>> unit = one;
	}
	attribute bearing : CyclicRatioScale {
		:>> unit = SI::'°';
		:>> modulus = 360;
	}
	attribute hourOfDay : CyclicRatioScale {
		:>> unit = SI::h;
		:>> modulus = 24;
	}
	attribute dBm : LogarithmicScale {
		:>> unit = one;
		:>> logarithmBase = 10;
		:>> factor = 10;
		:>> exponent = 1;
		:>> referenceQuantity = 0.001 [SI::W];
	}
	attribute octave : LogarithmicScale {
		:>> unit = one;
		:>> logarithmBase = 2;
		:>> factor = 1;
		:>> exponent = 1;
	}

	part def Probe {
		attribute hardness = 7 [mohs];
		attribute step = 1 [mohs];
		attribute top = 9 [mohs];
		attribute sameHardness : Boolean = hardness == 7 [mohs];
		attribute harder : Boolean = 9 [mohs] > hardness;
		attribute hardest = max(hardness, 9 [mohs]);
		attribute stillHardness = ConvertQuantity(hardness, mohs);
		attribute scaledHardness = 2 * hardness;
		attribute summedHardness = hardness + step;
		attribute offsetHardness = hardness + 1 [one];
		attribute hardnessSpread = top - hardness;
		attribute hardnessAsNumber = hardness == 7 [one];
		attribute hardnessInUnit = ConvertQuantity(hardness, one);
		attribute negatedHardness = -hardness;

		attribute heading = 450 [bearing];
		attribute sameHeading : Boolean = heading == 90 [bearing];
		attribute westward : Boolean = -60 [bearing] == 300 [bearing];
		attribute otherHeading : Boolean = heading != 91 [bearing];
		attribute notBelow : Boolean = heading < 90 [bearing];
		attribute notAbove : Boolean = heading > 90 [bearing];
		attribute atMost : Boolean = heading <= 90 [bearing];
		attribute below : Boolean = 10 [bearing] < 90 [bearing];
		attribute turned = heading + 30 ['°'];
		attribute turnedBack = heading - 100 [bearing];
		attribute turnedTwice = 2 * heading;
		attribute halved = heading / 2;
		attribute reversed = -heading;
		attribute inDegrees = ConvertQuantity(heading, '°');
		attribute onBearing = ConvertQuantity(3.0 ['°'], bearing);
		attribute sameAsAngle : Boolean = heading == 90 ['°'];
		attribute angleSame : Boolean = 810 ['°'] == heading;
		attribute rate = heading / 9 [s];
		attribute squared = heading * heading;
		attribute total = sum((heading, 30 [bearing]));
		attribute wake = 23 [hourOfDay] + 2 [h];
		attribute wakeAtOne : Boolean = wake == 1 [hourOfDay];
		attribute crossCycle = heading == 90 [hourOfDay];
		attribute mixedCycles = ConvertQuantity(1 [h], hourOfDay) == 25 [hourOfDay];

		attribute level = 30.0 [dBm];
		attribute quiet = 20.0 [dBm];
		attribute inWatts = ConvertQuantity(level, W);
		attribute backOnScale = ConvertQuantity(0.1 [W], dBm);
		attribute levelAboveWatt : Boolean = level > 1.0 [W];
		attribute levelIsOneWatt : Boolean = level == 1.0 [W];
		attribute wattIsLevel : Boolean = 1.0 [W] == level;
		attribute quieter : Boolean = 20.0 [dBm] < level;
		attribute louder = level + 3.0 [W];
		attribute doubledLevel = level * 2;
		attribute levelSpread = level - quiet;
		attribute noLevel = ConvertQuantity(-1.0 [W], dBm);
		attribute unanchored = ConvertQuantity(3.0 [octave], one);
	}
}
