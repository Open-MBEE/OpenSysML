// A magnitude on an interval scale (`26.85 [SI::'°C_abs']`) is a point on an
// affine scale, not a magnitude in a ratio unit. A point moves along its scale
// by a difference in the scale's unit (`°C`, a TemperatureDifferenceUnit whose
// factor on K is 1), from either side; two points differ by such a difference;
// a point on a scale placed on another scale moves and differs the same way,
// its magnitudes reaching kelvin through both placements. Everything else a
// ratio unit admits of a magnitude — a multiple, a product, a quotient, a
// power, a root, a negation, the sum of two, a fold over several — is refused
// by name of the scale and the operation, since ConvertQuantity(2 * warm, K)
// and 2 * ConvertQuantity(warm, K) would not agree. A magnitude in a ratio
// unit to the right of a point is a difference, kelvin included, since K is a
// TemperatureDifferenceUnit as much as a ThermodynamicTemperatureUnit; a
// magnitude minus a point is refused, since the magnitude would have to be
// read as a point on its unit's own scale for the difference to mean anything.
package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::*;
	private import QuantityCalculations::*;

	attribute <mK> millikelvin : TemperatureDifferenceUnit {
		:>> unitConversion : ConversionByPrefix { :>> prefix = milli; :>> referenceUnit = K; }
	}

	part def Probe {
		attribute warm : ThermodynamicTemperatureValue = ConvertQuantity(300.0 [K], SI::'°C_abs');
		attribute warmer : ThermodynamicTemperatureValue = warm + 10.0 [SI::'°C'];
		attribute cooler : ThermodynamicTemperatureValue = warm - 10.0 [SI::'°C'];
		attribute warmedInKelvin : ThermodynamicTemperatureValue = warm + 10.0 [K];
		attribute warmedFromLeft : ThermodynamicTemperatureValue = 10.0 [SI::'°C'] + warm;
		attribute warmedInMillikelvin : ThermodynamicTemperatureValue = warm + 500.0 [mK];
		attribute warmerInKelvin : ThermodynamicTemperatureValue = ConvertQuantity(warmer, K);
		attribute spread : TemperatureDifferenceValue = warm - 10.0 [SI::'°C_abs'];
		attribute cooledInKelvin : ThermodynamicTemperatureValue = warm - 20.0 [K];
		attribute zeroSpread : TemperatureDifferenceValue = warm - warm;
		attribute returned : ThermodynamicTemperatureValue = 10.0 [SI::'°C_abs'] + spread;
		attribute integerStep : ThermodynamicTemperatureValue = 20 [SI::'°C_abs'] + 3 [SI::'°C'];

		attribute room : IntervalScale {
			:>> unit = SI::'°C';
			:>> transformation : CoordinateFramePlacement { :>> source = SI::'°C_abs'; :>> origin = 20.0 [SI::'°C_abs']; }
		}
		attribute roomPoint : ThermodynamicTemperatureValue = 1.5 [room];
		attribute roomMoved : ThermodynamicTemperatureValue = roomPoint + 2.0 [K];
		attribute roomInKelvin : ThermodynamicTemperatureValue = ConvertQuantity(roomPoint, K);
		attribute roomSpread : TemperatureDifferenceValue = roomPoint - warm;
		attribute roomFromCelsius : TemperatureDifferenceValue = warm - roomPoint;
		attribute roomCooledInKelvin : ThermodynamicTemperatureValue = roomPoint - 1.5 [K];

		attribute doubled = 2 * warm;
		attribute halved = warm / 2;
		attribute timesSeconds = warm * 2.0 [s];
		attribute perPoint = 4.0 [J] / warm;
		attribute squared = warm ** 2;
		attribute rooted = sqrt(warm);
		attribute negated = -warm;
		attribute summed = warm + 10.0 [SI::'°C_abs'];
		attribute pointFromMagnitude = 300.0 [K] - warm;
		attribute pointFromDifference = 300.0 [SI::'°C'] - warm;

		attribute differences : TemperatureDifferenceValue[3] = (1.0 [SI::'°C'], 2.0 [K], 500.0 [mK]);
		attribute differenceSum : TemperatureDifferenceValue = differences->sum();
		attribute points : ThermodynamicTemperatureValue[2] = (warm, 10.0 [SI::'°C_abs']);
		attribute pointSum = points->sum();
		attribute pointProduct = points->product();
	}
}
