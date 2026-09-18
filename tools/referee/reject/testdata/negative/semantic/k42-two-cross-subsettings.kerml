// Invalid: a feature may own at most one cross subsetting (KerML 8.3.3.3 validateFeatureOwnedCrossSubsetting; pilot validateFeatureOwnedCrossSubsetting).
package K42TwoCrossSubsettings {
    class A {
        feature x : A;
        feature y : A;
    }
    assoc S {
        end a : A;
        end b : A crosses a.x crosses a.y;
    }
}
