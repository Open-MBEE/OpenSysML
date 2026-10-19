// `front : Panel :> frames` with `frames : ShapeItems::Shell` holds Shells too, so a
// binding to the Kernel member `edges` inherited only through Shell gets a slot.
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
