// A part performing an action whose required input nothing binds records the
// failure on the performance: its performer's other features still evaluate.
package test {
	private import ISQ::*;
	private import SI::*;

	action def ApplyHeat { in energy[1..*] : ISQ::EnergyValue; }
	part def Toaster {
		attribute cycleTime : ISQ::DurationValue;
		perform action applyHeat : ApplyHeat;
	}
	part slow : Toaster { attribute :>> cycleTime = 200.0 [SI::s]; }
}
