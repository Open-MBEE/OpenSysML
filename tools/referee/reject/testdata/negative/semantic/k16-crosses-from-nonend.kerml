// Invalid: a cross subsetting must be owned by an end feature of a type with two or more ends (KerML 8.3.3.3 validateCrossSubsettingCrossingFeature; pilot validateCrossSubsettingCrossingFeature).
package K16CrossesFromNonend {
    class A {
        feature p : A;
        feature q : A crosses p;
    }
}
