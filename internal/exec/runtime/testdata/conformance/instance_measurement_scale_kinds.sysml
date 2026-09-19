// Only an IntervalScale relates its points by differences in its unit. A point
// on an ordinal, cyclic ratio or logarithmic scale is still a point, so it is
// no ratio magnitude: it has no multiple, no sum, and no difference or order
// against another reference, and the error names the scale and its kind. Two
// points on the same such scale still compare, and a point on any scale stays
// itself when asked for as written.
package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::*;
	private import QuantityCalculations::*;

	attribute mohs : OrdinalScale {
		:>> unit = SI::K;
	}
	attribute compass : CyclicRatioScale {
		:>> unit = SI::rad;
		:>> modulus = 6.283185307179586;
	}
	attribute decibel : LogarithmicScale {
		:>> unit = SI::W;
		:>> logarithmBase = 10;
		:>> factor = 10;
		:>> exponent = 1;
	}

	part def Probe {
		attribute hardness = 7 [mohs];
		attribute sameHardness : Boolean = hardness == 7 [mohs];
		attribute harder : Boolean = 9 [mohs] > hardness;
		attribute stillHardness = ConvertQuantity(hardness, mohs);
		attribute scaledHardness = 2 * hardness;
		attribute summedHardness = hardness + 1 [mohs];
		attribute offsetHardness = hardness + 1 [K];
		attribute hardnessSpread = 9 [mohs] - hardness;
		attribute hardnessInKelvin = hardness == 7 [K];
		attribute negatedHardness = -hardness;

		attribute heading = 1.0 [compass];
		attribute turned = heading + 0.5 [rad];
		attribute turnedTwice = 2 * heading;
		attribute headingSpread = heading - 0.5 [compass];
		attribute headingBelow : Boolean = heading < 2.0 [compass];

		attribute level = 30.0 [decibel];
		attribute louder = level + 3.0 [W];
		attribute doubledLevel = level * 2;
		attribute levelSpread = level - 20.0 [decibel];
		attribute levelAgainstWatts = level > 1.0 [W];
	}
}
