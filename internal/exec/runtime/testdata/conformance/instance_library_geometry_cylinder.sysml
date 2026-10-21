// A Cylinder inherits Items::Item through the Geometry domain library: the base
// and lateral face edges resolve through the `binding [1]` partial bindings, and
// the curved face `cf`, a Surface subsetting `faces : StructuredSurface`, carries
// the `edges` those bindings connect.
package test {
    private import ShapeItems::*;
    private import ISQ::*;
    private import SI::*;

    part def Drum {
        part cyl : Cylinder {
            :>> semiMajorAxis = 4 [m];
            :>> semiMinorAxis = 3 [m];
            :>> height = 2 [m];
        }
    }
}
