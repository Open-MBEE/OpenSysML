// Invalid: a connector's related features must be featured within its featuring types; here the implicit binding connector of a feature reference expression (KerML 8.3.4.1 validateConnectorTypeFeaturing; pilot validateConnectorTypeFeaturing).
package K27ConnectorFeatureOutsideFeaturingType {
    class A {
        feature x : A;
    }
    class B {
        feature y : A = A::x;
    }
}
