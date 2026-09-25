// Invalid: a concrete association must have at least two related types (KerML 8.3.3.5 validateAssociationRelatedTypes; pilot validateAssociationRelatedTypes).
package K25AssocOneEnd {
    class T;
    assoc A {
        end a : T;
    }
}
