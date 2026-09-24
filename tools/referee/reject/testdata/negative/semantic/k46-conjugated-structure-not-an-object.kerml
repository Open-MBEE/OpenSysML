// Invalid: a classifier must directly or indirectly specialize its kind's default supertype, and a structure conjugating a plain class reaches Base::Anything but not Objects::Object (KerML 8.3.4.3.2 checkStructureSpecialization; pilot validateClassifierDefaultSupertype).
package K46ConjugatedStructureNotAnObject {
    class D;
    struct S ~ D;
    behavior B;
    function F ~ B;
}
