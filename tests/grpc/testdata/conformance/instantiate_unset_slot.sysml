package test {
    private import ScalarValues::*;

    part def Engine {
        attribute power : Real = 120.0;
    }

    part def Vehicle {
        attribute d : Real;
        attribute k : Real = 2.0;
        part engine : Engine;
    }
}
