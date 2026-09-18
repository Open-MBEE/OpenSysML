// Invalid: a cross feature must have the same type as its end feature (KerML 8.3.3.3 validateFeatureCrossFeatureType; pilot validateFeatureCrossFeatureType).
package K18CrossFeatureTypeDiffers {
    class A {
        feature x : A;
    }
    class B;
    assoc S {
        end a : A;
        end b : B crosses a.x;
    }
}
