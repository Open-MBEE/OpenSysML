// Census: validateFeatureChainingFeaturesNotSelf
// Expect: error: Feature cannot have itself in a feature chain
package K12ChainThroughSelf {
    class C {
        feature a : C;
        feature b chains a.b;
    }
}
