// A quantity an operation composes is re-expressed in the coherent unit the
// library declares for its dimension: a product or quotient of named units, a
// prefixed unit whose scale folds into the magnitude, a fractional power, and a
// derived unit the model declares through its `unitPowerFactors`. A result that
// reduces to dimension one stays a bare number, and a product the library
// declares no unit for keeps its base-unit form, as does one over a dimension-one
// unit; a quantity raised to a quantity is refused as before.
package test {
	private import SI::*;
	private import ISQ::*;
	private import MeasurementReferences::*;

	attribute def GravParam :> Quantities::ScalarQuantityValue;

	attribute 'm³⋅s⁻²' : DerivedUnit {
		private attribute m3 : UnitPowerFactor[1] { :>> unit = SI::m; :>> exponent = 3; }
		private attribute s_2 : UnitPowerFactor[1] { :>> unit = SI::s; :>> exponent = -2; }
		attribute :>> unitPowerFactors = (m3, s_2);
	}

	part def Orbit {
		attribute gravity : AccelerationValue = 9.80665 ['m⋅s⁻²'];
		attribute burn : TimeValue = 311 [s];
		attribute speed : SpeedValue = gravity * burn;

		attribute thrust : ForceValue = 10 [N];
		attribute propellant : MassValue = 2 [kg];
		attribute perMass : AccelerationValue = thrust / propellant;

		attribute mu : GravParam = 3.986E14 ['m³⋅s⁻²'];
		attribute radiusKm : LengthValue = 6563 [km];
		attribute radiusM : LengthValue = 6563000 [m];
		attribute orbitalKm : SpeedValue = (mu / radiusKm) ^ (1/2);
		attribute orbitalM : SpeedValue = (mu / radiusM) ^ (1/2);

		attribute lightThrust : ForceValue = 10 [mN];
		attribute lightMass : MassValue = 2 [g];
		attribute perLightMass : AccelerationValue = lightThrust / lightMass;

		attribute work : EnergyValue = 4 [kJ] / 2 [km] * 1 [m];
		attribute torque : TorqueValue = 3 [N] * 2 [m];
		attribute ratio = radiusM / radiusKm;
		attribute halfTorque : TorqueValue = 3 [J];
		attribute unlisted = 2 [kg] * 3 [K];
		attribute swept = 2 [m] * 3 [rad];
		attribute unknownPower = 2 [m] ^ 3 [s];
	}
}
