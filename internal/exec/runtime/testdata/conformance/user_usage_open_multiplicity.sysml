// A usage the model itself declares with an open multiplicity reads closed over the
// model, at model level exactly as on an instantiated object, by the rule the library
// collections follow: its multiplicity bounds how many values it has on each instance
// of its featuring type (KerML 1.0 §7.3.2.2, §7.4.12) and a feature's values are what
// the model gives it (§7.3.4.4 subsetting, §7.4.11 feature values), so a lower bound of
// 0 that nothing declared meets is met by no value — `wheels[0..*]`, `cargo[0..4]`,
// `spares[*]` and `nested[0..*] { part hub; }` are empty — while a lower bound of 1 or
// more that nothing meets (`w[1..*]`, `r[2..4]`) leaves the count undetermined, carrying
// the bounds and what they fix (`w` is not empty). A feature value (`= (w1, w2)`) and a
// subsetting usage meeting the bound (`fl :> fitted[1..*]`) close the count. A parameter of
// a behavior usage nothing has invoked (`act.i`, `act.o`) is an unbound input whatever its
// multiplicity: its caller supplies it (SysML v2 §7.17.2, KerML 1.0 §7.4.7); a directed
// feature of a port or part (`dir.inp`, `dir.pt.v`) is no parameter and reads closed.
// Subsetting is transitive (§7.3.3.1): `leaf :> l2 :> l1` makes `l1` hold `leaf`, and
// `kleaf :> k2 :> k1[1..*]` meets `k1`'s bound.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part def Wheel { attribute mass : Real = 1.0; }
	action def Act { in i : Integer[0..1]; out o : Integer[0..1]; }
	port def Pt { in attribute v : Integer[0..1]; out item o : Box[0..*]; }
	part def Dir { in attribute inp : Integer[0..1]; out attribute outp : Integer[0..*]; port pt : Pt; }
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
		action act : Act;
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
	part dir : Dir;
	attribute dirIn = dir.inp;
	attribute dirOut = dir.outp;
	attribute portIn = dir.pt.v;
	attribute portOut = dir.pt.o;
	part l1 : Wheel[0..*];
	part l2 : Wheel[0..*] :> l1;
	part leaf : Wheel :> l2;
	part k1 : Wheel[1..*];
	part k2 : Wheel[0..*] :> k1;
	part kleaf : Wheel :> k2;
	attribute nL1 = size(l1);
	attribute nK1 = size(k1);
	attribute actIn = car.act.i;
	attribute actOut = car.act.o;

	attribute total : Integer = size(car.wheels) + size(wheels) + size(car.nested.hub) + size(fitted);
	calc probe { out one : Integer = 1; }
}
