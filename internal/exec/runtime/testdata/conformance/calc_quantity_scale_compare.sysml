// A point on an interval scale compares with a magnitude in a ratio unit, or
// with a point on another scale, as ConvertQuantity relates them: the right
// operand is carried onto the left operand's reference through the scale's
// anchor, so `==` agrees with ConvertQuantity about which values are the same
// temperature, and `<` orders 300 K below 30 °C on the absolute Celsius scale.
// A point on a scale placed on another scale compares through both placements.
// max and min are comparisons, so they pick between a point and a magnitude,
// returning the winning operand as written. A set of quantities holds one
// member per temperature, however each member is written. A scale with no
// anchor compares with itself only.
package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::*;
	private import QuantityCalculations::*;
	private import Collections::*;
	private import CollectionFunctions::*;
	private import SequenceFunctions::*;

	part def Probe {
		attribute warm : ThermodynamicTemperatureValue = ConvertQuantity(300.0 [K], SI::'°C_abs');
		attribute sameAsKelvin : Boolean = 300.0 [K] == warm;
		attribute sameAsKelvinFlipped : Boolean = warm == 300.0 [K];
		attribute notKelvin : Boolean = 300.0 [K] != warm;
		attribute samePoint : Boolean = 20.0 [SI::'°C_abs'] == 293.15 [K];
		attribute otherPoint : Boolean = warm != 20.0 [SI::'°C_abs'];
		attribute kelvinBelow : Boolean = 300.0 [K] < 30.0 [SI::'°C_abs'];
		attribute kelvinAbove : Boolean = 300.0 [K] > 30.0 [SI::'°C_abs'];
		attribute pointAbove : Boolean = warm > 20.0 [SI::'°C_abs'];
		attribute pointAboveKelvin : Boolean = warm >= 299.0 [K];
		attribute pointBelowKelvin : Boolean = warm <= 299.0 [K];
		attribute freezingIsZero : Boolean = 0.0 [SI::'°C_abs'] == 273.15 [K];
		attribute aboveZero : Boolean = warm > 0;
		attribute zeroBelow : Boolean = 0 < warm;

		attribute room : IntervalScale {
			:>> unit = SI::'°C';
			:>> transformation : CoordinateFramePlacement { :>> source = SI::'°C_abs'; :>> origin = 20.0 [SI::'°C_abs']; }
		}
		attribute roomPoint : ThermodynamicTemperatureValue = ConvertQuantity(warm, room);
		attribute roomIsWarm : Boolean = roomPoint == warm;
		attribute warmIsRoom : Boolean = warm == roomPoint;
		attribute roomIsKelvin : Boolean = roomPoint == 300.0 [K];
		attribute kelvinIsRoom : Boolean = 300.0 [K] == roomPoint;
		attribute roomBelow : Boolean = 5.0 [room] < warm;

		attribute warmest : ThermodynamicTemperatureValue = max(warm, 300.5 [K]);
		attribute coolest : ThermodynamicTemperatureValue = min(warm, 300.5 [K]);
		attribute warmestPoint : ThermodynamicTemperatureValue = max(20.0 [SI::'°C_abs'], warm);
		attribute coolestAcrossScales : ThermodynamicTemperatureValue = min(roomPoint, 27.0 [SI::'°C_abs']);
		attribute warmestOfMany : ThermodynamicTemperatureValue = max(max(warm, 20.0 [SI::'°C_abs']), 305.0 [K]);

		attribute temperatures : Set { :>> elements = (300.0 [K], warm, roomPoint, 20.0 [SI::'°C_abs']); }
		attribute distinctCount : Natural = size(temperatures);
		attribute holdsWarm : Boolean = contains(temperatures, 300.0 [K]);
		attribute holdsCold : Boolean = contains(temperatures, 293.15 [K]);
		attribute holdsNeither : Boolean = contains(temperatures, 310.0 [K]);
		attribute sequenceHoldsWarm : Boolean = includes((300.0 [K], 310.0 [K]), warm);
		attribute sequenceHoldsCold : Boolean = includes((300.0 [K], 310.0 [K]), 20.0 [SI::'°C_abs']);

		attribute utcAgainstSeconds = 5.0 [Time::UTC] == 5.0 [s];
		attribute utcAgainstUtc : Boolean = 5.0 [Time::UTC] == 5.0 [Time::UTC];
		attribute utcOrdered : Boolean = 3.0 [Time::UTC] < 5.0 [Time::UTC];
	}
}
