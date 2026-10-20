// A usage the model itself declares with an open multiplicity reads closed over the
// model, at model level exactly as on an instantiated object, by the rule the library
// collections follow: its multiplicity bounds how many values it has on each instance
// of its featuring type (KerML 1.0 §7.3.2.2, §7.4.12) and a feature's values are what
// the model gives it (§7.3.4.4 subsetting, §7.4.11 feature values), so a lower bound of
// 0 that nothing declared meets is met by no value — `wheels[0..*]`, `cargo[0..4]`,
// `spares[*]` and `nested[0..*] { part hub; }` are empty — while a lower bound of 1 or
// more that nothing meets (`w[1..*]`, `r[2..4]`) leaves the count undetermined, carrying
// the bounds and what they fix (`w` is not empty). A feature value (`= (w1, w2)`) and a
// subsetting usage meeting the bound (`fl :> fitted[1..*]`) close the count.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part def Wheel { attribute mass : Real = 1.0; }
	item def Box;
	part def Car {
		part wheels : Wheel[0..*];
		part w : Wheel[1..*];
		item cargo : Box[0..4];
		ref part spares : Wheel[*];
		part r : Wheel[2..4];
		part nested : Wheel[0..*] { part hub : Wheel; }
		part w1 : Wheel;
		part w2 : Wheel;
		part bound : Wheel[0..*] = (w1, w2);
		part fitted : Wheel[1..*];
		part fl : Wheel :> fitted;
		attribute nWheels : Integer = size(wheels);
		attribute emptyWheels : Boolean = isEmpty(wheels);
		attribute hasW1 : Boolean = includes(wheels, w1);
		attribute nNested : Integer = size(nested);
		attribute nBound : Integer = size(bound);
		attribute nFitted : Integer = size(fitted);
		attribute nW : Integer = size(w);
		attribute emptyW : Boolean = isEmpty(w);
		attribute nR : Integer = size(r);
		constraint noWheels { size(wheels) == 0 }
	}
	part car : Car;
	part wheels : Wheel[0..*];
	part w : Wheel[1..*];
	item cargo : Box[0..4];
	part nested : Wheel[0..*] { part hub : Wheel; }
	part fitted : Wheel[1..*];
	part fl : Wheel :> fitted;

	attribute wheelsModel = car.wheels;
	attribute cargoModel = car.cargo;
	attribute sparesModel = car.spares;
	attribute nestedModel = car.nested;
	attribute hubModel = car.nested.hub;
	attribute nWheelsModel = car.nWheels;
	attribute emptyWheelsModel = car.emptyWheels;
	attribute hasW1Model = car.hasW1;
	attribute nNestedModel = car.nNested;
	attribute nBoundModel = car.nBound;
	attribute nFittedModel = car.nFitted;
	attribute noWheelsModel = car.noWheels;
	attribute wModel = car.w;
	attribute nWModel = car.nW;
	attribute emptyWModel = car.emptyW;
	attribute rModel = car.r;
	attribute nRModel = car.nR;
	attribute pkgWheels = wheels;
	attribute pkgCargo = cargo;
	attribute pkgNested = nested;
	attribute pkgW = w;
	attribute pkgFitted = fitted;

	calc probe { out one : Integer = size(car.wheels) + size(wheels) + size(car.nested.hub) + size(fitted); }
}
