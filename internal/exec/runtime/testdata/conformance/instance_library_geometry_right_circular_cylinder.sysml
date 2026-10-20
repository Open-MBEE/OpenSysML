// A part whose `shape` is a ShapeItems::RightCircularCylinder with its height and radius
// given: the library's zero offsets (`:>> xoffset { :>> num = 0; }` over the package-level
// `xoffset ... default 0 [m]`) read as lengths, the base circle's edge length is derived
// from a radius the library values by nothing (its semi-axes are bound from the radius,
// never the radius from them), and instantiation reads every feature without a finding.
package test {
    part def P {
        item :>> shape : ShapeItems::RightCircularCylinder {
            :>> height = 20 [SI::cm];
            :>> radius = 60 [SI::cm];
        }
    }
    part p : P;
}
