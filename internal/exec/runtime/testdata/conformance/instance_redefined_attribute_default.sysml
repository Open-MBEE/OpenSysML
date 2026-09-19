// A redefining attribute is the attribute it redefines declared again, so it
// holds the value that declaration wrote and both names read it.
package test {
    private import ScalarValues::Real;

    part def Vehicle {
        attribute mass : Real = 1000.0;
        attribute payload : Real = 250.0;
    }

    part def Truck :> Vehicle {
        attribute grossMass :>> mass;
        attribute :>> payload = 400.0;
    }

    part truck : Truck;
}
