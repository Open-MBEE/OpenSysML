// A default whose value count does not conform to its multiplicity is still reported,
// beside an unset quantity attribute and nested under a part carrying one alike.
package test {
    private import ISQ::*;
    private import SI::*;
    private import ScalarValues::*;

    attribute def Point {
        attribute x : Real = 1.0;
        attribute xs : Real[3] = x;
    }

    part def Engine {
        attribute mass :> ISQ::mass;
        attribute origin : Point;
    }

    part def Rocket {
        part engine : Engine;
        attribute mass :> ISQ::mass = 100 [kg];
        attribute masses : MassValue[3] = mass;
    }
}
