// A valueless attribute of a library value type holds no value, while its
// valued sibling and a part typed by a class still hold what they materialize.
package test {
    private import ScalarValues::*;

    part def Engine {
        attribute power : Real = 120.0;
    }

    attribute def Point {
        attribute x : Real = 1.0;
    }

    part def Vehicle {
        attribute d : Real;
        attribute k : Real = 2.0;
        attribute names : String[3];
        attribute origin : Point;
        part engine : Engine;
    }
}
