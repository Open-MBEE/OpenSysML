// An explicit execution of the action the object's performance performs
// succeeds: a parameter writing no multiplicity may hold no value.
package test {
	private import ISQ::*;
	private import SI::*;

	action def ApplyHeat { in energy : ISQ::EnergyValue; }
	action def ToastBread { action applyHeat : ApplyHeat; }
}
