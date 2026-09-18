// Invalid: a binding connector is binary, so its ends must reference exactly two features (KerML 8.3.4.7.2 validateBindingConnectorIsBinary; pilot validateBindingConnectorIsBinary).
package K44BindingConnectorWithThreeEnds {
    class A;
    class C {
        feature x : A; feature y : A; feature z : A;
        binding b of x = y { end feature e references z; }
        abstract binding b2;
    }
}
