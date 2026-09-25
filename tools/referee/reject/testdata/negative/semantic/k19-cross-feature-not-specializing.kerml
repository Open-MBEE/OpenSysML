// Invalid: a cross feature must specialize the cross features of redefined ends (KerML 8.3.3.3 validateFeatureCrossFeatureSpecialization; pilot validateFeatureCrossFeatureSpecialization).
package K19CrossFeatureNotSpecializing {
    class A {
        feature x : A;
        feature y : A;
    }
    assoc S {
        end a : A;
        end b : A crosses a.x;
    }
    assoc T specializes S {
        end a2 : A redefines a;
        end b2 : A redefines b crosses a2.y;
    }
}
