// Invalid: a feature bound with `=` may not be given another value by a redefining feature; only a `default =` value may be overridden (KerML 8.3.4.10.2; pilot validateFeatureValueOverriding).
package S46FeatureValueOverriding {
    private import ScalarValues::Integer;
    part def P {
        attribute a : Integer = 1;
    }
    part p : P {
        attribute :>> a = 2;
    }
    part def Q :> P {
        attribute :>> a = 3;
    }
}
