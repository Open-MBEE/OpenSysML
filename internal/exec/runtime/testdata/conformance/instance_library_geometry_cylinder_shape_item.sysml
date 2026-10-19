// A ShapeItems::Cylinder's curved face `cf : Surface :> faces` carries StructuredSurface's
// `edges`, which the library binds, so instantiation reads every feature without a missing-feature error.
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
