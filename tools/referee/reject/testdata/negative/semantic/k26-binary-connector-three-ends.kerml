// Invalid: a connector typed by BinaryLink cannot have more than two ends (KerML 8.3.4.1 validateConnectorBinarySpecialization; pilot validateConnectorBinarySpecialization).
package K26BinaryConnectorThreeEnds {
    class T {
        feature x : T;
        feature y : T;
        feature z : T;
        connector c : Links::BinaryLink (x, y, z);
    }
}
