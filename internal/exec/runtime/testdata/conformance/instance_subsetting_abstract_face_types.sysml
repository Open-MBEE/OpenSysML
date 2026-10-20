// A feature subsetting an abstract collection of another type holds objects of both types
// (KerML 1.0 §7.3.4.3–§7.3.4.4): `front : Panel :> faces` with `faces : Face [0..*]` holds a Panel
// that is a Face, judged by `istype`, `@`, `hastype` and `as` alike, while a Panel held by
// nothing of the sort is no Face.
package test {
    item def Edge;
    abstract item def Face {
        item edges : Edge [0..*];
    }
    item def Panel {
        attribute area = 2.0;
    }
    item def Rig {
        abstract item faces : Face [0..*];
        item front : Panel :> faces;
        item spare : Panel;
        attribute frontIsFace = front istype Face;
        attribute frontAtFace = front @ Face;
        attribute frontHasFace = front hastype Face;
        attribute frontIsPanel = front istype Panel;
        attribute frontHasPanel = front hastype Panel;
        attribute frontIsEdge = front istype Edge;
        attribute frontAtEdge = front @ Edge;
        attribute spareIsFace = spare istype Face;
        attribute facesAreFaces = faces istype Face;
        attribute frontArea = front.area;
        item frontAsFace [1] = front as Face;
        item frontAsEdge [0..1] = front as Edge;
    }
    item rig : Rig;
}
