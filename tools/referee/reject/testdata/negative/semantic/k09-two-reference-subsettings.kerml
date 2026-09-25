// Invalid: a feature may own at most one reference subsetting (KerML 8.3.3.3 validateFeatureOwnedReferenceSubsetting; pilot validateFeatureOwnedReferenceSubsetting).
package K09TwoReferenceSubsettings {
    class C {
        feature a : C;
        feature b : C;
        feature c references a references b;
    }
}
