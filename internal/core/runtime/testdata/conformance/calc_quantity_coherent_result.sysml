// A calculation's result is bound to the quantity kind its return declares in
// the coherent unit the library states for it: a product of an acceleration and
// a time is a speed in `m/s`, a quotient of a force and a mass an acceleration in
// `m⋅s⁻²`, and the square root of a ratio over a prefixed length folds the prefix
// scale into the magnitude.
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

	calc def velocity { in a :> ISQ::acceleration; in t :> ISQ::time; return v :> ISQ::speed = a * t; }
	calc def orbital { in mu : GravParam; in r :> ISQ::length; return v :> ISQ::speed = (mu / r) ^ (1/2); }
	calc def perMass { in f :> ISQ::force; in m :> ISQ::mass; return a :> ISQ::acceleration = f / m; }
	calc def check { in v :> ISQ::speed; return ok : ScalarValues::Boolean = v == 5 [SI::'m/s']; }

	calc def Flight {
		out burn : SpeedValue = velocity(9.80665 ['m⋅s⁻²'], 311 [s]);
		out orbitKm : SpeedValue = orbital(3.986E14 ['m³⋅s⁻²'], 6563 [km]);
		out orbitM : SpeedValue = orbital(3.986E14 ['m³⋅s⁻²'], 6563000 [m]);
		out thrust : AccelerationValue = perMass(10 [N], 2 [kg]);
		out lightThrust : AccelerationValue = perMass(10 [mN], 2 [g]);
		out same : ScalarValues::Boolean = check(velocity(2.5 ['m⋅s⁻²'], 2 [s]));
	}
	calc flight : Flight;
}
