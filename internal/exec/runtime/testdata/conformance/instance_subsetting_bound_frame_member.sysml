// `front : Panel :> faces` with `faces : StructuredSurface` holds StructuredSurfaces too, so a
// binding to the Kernel member `edges`, which only the conjoined type declares, gets a slot.
package test {
    private import ScalarValues::*;

    item def Panel {
        attribute thickness : Real = 2.0;
    }
    part def Holder {
        item faces : Objects::StructuredSpaceObject::StructuredSurface [0..*];
        item front : Panel [1] :> faces;
        item outline : Objects::StructuredSpaceObject::StructuredCurve [1];
        item rim : Objects::StructuredSpaceObject::StructuredCurve [1];
        item spare : Objects::StructuredSpaceObject::StructuredCurve [2] = (outline, rim);
        binding front.edges = spare;
        attribute frontThickness : Real = front.thickness;
    }
    part holder : Holder;
}
