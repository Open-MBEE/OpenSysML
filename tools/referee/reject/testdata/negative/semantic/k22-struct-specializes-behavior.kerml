// Invalid: a structure cannot specialize a behavior (KerML 8.3.3.4.3 validateStructureSpecialization; pilot validateStructureSpecialization).
package K22StructSpecializesBehavior {
    behavior B;
    struct S specializes B;
}
