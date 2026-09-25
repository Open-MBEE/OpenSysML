// A part or item def inherits Items::Item's features; the model's own isSolid masks
// the library's derivation, and the Kernel frame (self, start :>> startShot) stays out.
package test {
    private import ScalarValues::*;

    item def Cargo {
        attribute mass : Real = 3.0;
    }

    part def Engine;

    part def Truck {
        attribute mass : Real = 1000.0;
        part engine : Engine;
        item cargo : Cargo;
        attribute :>> isSolid = false;
    }
}
