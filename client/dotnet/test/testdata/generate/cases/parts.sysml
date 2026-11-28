// Definitions the vehicles document refers to from another file.
package CaseParts {
    private import ScalarValues::*;

    part def Engine {
        attribute power : Real default 100.0;
        attribute cylinders : Integer default 4;
    }

    part def BigEngine :> Engine {
        attribute :>> power default 500.0;
    }

    enum def Fuel { petrol; diesel; electric; }
}
