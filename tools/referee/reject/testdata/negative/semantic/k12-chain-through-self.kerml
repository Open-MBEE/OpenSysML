// Invalid: a feature cannot have itself in its feature chain (KerML 8.3.3.3 validateFeatureChainingFeaturesNotSelf; pilot validateFeatureChainingFeaturesNotSelf).
package K12ChainThroughSelf {
    class C {
        feature a : C;
        feature b chains a.b;
    }
}
