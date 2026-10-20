// Invalid: a `member` binding connector ahead of an end's cross feature is not the owned cross feature, so the mistyped feature after it must still have the end's type (KerML 8.3.3.3.4 Feature::ownedCrossFeature, validateFeatureCrossFeatureType; pilot validateFeatureCrossFeatureType).
package K76CrossFeatureAfterMemberBindingTypeDiffers {
    class A;
    class B;
    class C;
    assoc Q {
        end feature a : A[1] {
            feature x : A[1];
            feature y : A[1];
            member binding bx of x = y;
            member feature c : C[0..*];
        }
        end feature b : B[1];
    }
}
