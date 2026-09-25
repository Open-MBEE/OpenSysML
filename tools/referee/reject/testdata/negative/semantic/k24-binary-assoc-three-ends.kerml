// Invalid: an association specializing BinaryLink cannot have more than two ends (KerML 8.3.3.5 validateAssociationBinarySpecialization; pilot validateAssociationBinarySpecialization).
package K24BinaryAssocThreeEnds {
    class T;
    assoc A specializes Links::BinaryLink {
        end a : T;
        end b : T;
        end c : T;
    }
}
