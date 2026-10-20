// An object held by a feature that subsets a library portion feature is of every type the
// feature inherits (KerML 1.0 §7.3.4.3–§7.3.4.4): `front : Panel :> faces` holds a Panel that is
// also a StructuredSurface, so `istype`, `@` and `as` answer for both and `hastype` names the
// direct types only.
package test {
    item def Panel;
    item def Rig :> Objects::StructuredSpaceObject {
        item front : Panel :> faces;
        item spare : Panel;
        attribute frontIsStructuredSurface = front istype Objects::StructuredSpaceObject::StructuredSurface;
        attribute frontIsSurface = front istype Objects::Surface;
        attribute frontAtStructuredSurface = front @ Objects::StructuredSpaceObject::StructuredSurface;
        attribute frontIsPanel = front istype Panel;
        attribute frontHasPanel = front hastype Panel;
        attribute frontHasStructuredSurface = front hastype Objects::StructuredSpaceObject::StructuredSurface;
        attribute frontHasSurface = front hastype Objects::Surface;
        attribute frontIsStructuredCurve = front istype Objects::StructuredSpaceObject::StructuredCurve;
        attribute spareIsStructuredSurface = spare istype Objects::StructuredSpaceObject::StructuredSurface;
        attribute facesAreStructuredSurfaces = faces istype Objects::StructuredSpaceObject::StructuredSurface;
        item frontAsSurface [1] = front as Objects::Surface;
    }
    item rig : Rig;
}
