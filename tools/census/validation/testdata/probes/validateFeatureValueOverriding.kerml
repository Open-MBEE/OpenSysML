// Census: validateFeatureValueOverriding
// Expect: error: cannot override the binding value
package P {
    private import ScalarValues::Integer;
    class C {
        feature a : Integer = 1;
    }
    class D specializes C {
        feature :>> a = 2;
    }
}
