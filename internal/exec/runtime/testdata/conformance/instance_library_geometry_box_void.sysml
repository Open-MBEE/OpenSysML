// A Box carrying a Cylinder void: isSolid derives false from the one void, and
// the void's faces resolve through the same partial bindings as a plain one.
package test {
    private import ShapeItems::*;
    private import ISQ::*;
    private import SI::*;

    part def Casting {
        part block : Box {
            :>> length = 2 [m];
            :>> width = 1 [m];
            :>> height = 3 [m];
            item :>> voids : Cylinder [1] {
                :>> semiMajorAxis = 0.5 [m];
                :>> semiMinorAxis = 0.5 [m];
                :>> height = 3 [m];
            }
        }
    }
}
