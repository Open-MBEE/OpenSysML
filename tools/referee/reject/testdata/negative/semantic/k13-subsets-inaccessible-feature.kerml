// Invalid: a subsetted feature must be accessible from the subsetting feature (KerML 8.3.3.3 validateSubsettingFeaturingTypes; pilot validateSubsettingFeaturingTypes).
package K13SubsetsInaccessibleFeature {
    class A {
        feature a : A;
    }
    class B {
        feature b : A subsets A::a;
    }
}
