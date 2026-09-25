// A Box inherits Items::Item through the Geometry domain library: isSolid derives from
// the empty voids, shape is optional and unbound, and the Kernel frame stays out.
package test {
    private import ShapeItems::*;
    private import ISQ::*;
    private import SI::*;

    part def Crate {
        part box : Box {
            :>> length = 2 [m];
            :>> width = 1 [m];
            :>> height = 1 [m];
        }
    }
}
