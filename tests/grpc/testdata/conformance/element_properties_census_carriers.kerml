// Carriers of the KerML-only relationships the census finds in no other fixture: the
// statement forms of a disjoining and a feature inverting, which the element form states.
package CensusCarriersKerML {
    class Vehicle;
    class Mineral;
    disjoint Mineral from Vehicle;
    classifier Person {
        feature vehicles : Vehicle[*];
        feature owners : Person[*];
        inverse owners of vehicles;
    }
}
