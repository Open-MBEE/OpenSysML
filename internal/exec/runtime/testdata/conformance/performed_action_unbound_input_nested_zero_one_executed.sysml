// An explicit execution of the action an object's performance failed is refused
// for the same unbound input the performance recorded.
package test {
	private import ISQ::*;
	private import SI::*;

	action def ApplyHeat { in energy[0..1] : ISQ::EnergyValue; }
	action def ToastBread { action applyHeat : ApplyHeat; }
}
