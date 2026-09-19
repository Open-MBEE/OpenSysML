// An attribute redefinition reading an analysis usage's output runs the case
// (the training corpus's fuel-economy pattern): the usage's subject is the part
// whose attribute reads it, and the read goes through the part's feature.
package test {
	private import ScalarValues::*;

	part def Vehicle {
		attribute mass : Real = 100.0;
		attribute fuelEconomy : Real;
	}

	analysis def FuelEconomy {
		subject v : Vehicle;
		out fe : Real = v.mass / 2.0;
	}

	part vehicle : Vehicle {
		attribute :>> fuelEconomy = cityAnalysis.fe;
	}

	analysis cityAnalysis : FuelEconomy {
		subject v = vehicle;
	}

	attribute economyRead = vehicle.fuelEconomy;
}
