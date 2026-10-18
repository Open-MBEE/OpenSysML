// A part whose `shape` is a ShapeItems::Cylinder: the cylinder's curved face
// `cf` is declared a Surface subsetting `faces : StructuredSurface`, so it
// carries StructuredSurface's `edges`, which the library binds to the cylinder's
// own edges. Instantiation reads every feature without a missing-feature error.
package test {
    part def P {
        item :>> shape : ShapeItems::Cylinder {
            :>> height = 20 [SI::cm];
            :>> base.shape : ShapeItems::Circle {
                :>> radius = 60 [SI::cm];
            }
        }
    }
    part p : P;
}
