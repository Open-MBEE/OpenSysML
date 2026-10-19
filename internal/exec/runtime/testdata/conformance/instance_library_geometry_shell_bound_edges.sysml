// A feature typed by a model definition that subsets a feature typed by the
// library's `ShapeItems::Shell` holds objects that are Shells too, so a binding
// to the Kernel member `edges` the object inherits only through the Shell resolves
// to a slot of the object instead of failing as a missing feature.
package test {
    private import ScalarValues::*;

    item def Panel {
        attribute area : Real = 2.0;
    }
    part def Holder {
        item frames : ShapeItems::Shell [0..*];
        item front : Panel [1] :> frames;
        item outline : ShapeItems::Polygon [1];
        item rim : ShapeItems::Polygon [1];
        item curves : ShapeItems::Polygon [2] = (outline, rim);
        binding front.edges = curves;
    }
    part holder : Holder;
}
