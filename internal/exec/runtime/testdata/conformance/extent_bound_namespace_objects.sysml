// A namespace-level usage given a value is bound to that value for the run (KerML 1.0 §7.4.11:
// a feature value binds the feature to its expression's result), so `all Car` reaches the
// object its constructor made before any read of it, a read of it twice yields that one
// object, and a usage bound to another denotes that other object rather than a second one.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Car;
    part def Truck :> Car;

    ref part car : Car = new Car();
    part fleet = new Truck();
    part spare : Car;
    ref part alias : Car = spare;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Car) == 3,
            size(all Truck) == 1,
            car === car,
            fleet === fleet,
            alias === spare,
            includes(all Car, car),
            includes(all Truck, fleet),
            (all Car)#(3) === spare
        );
    }
}
