// Invalid: a feature chain needs at least two chaining features (KerML 8.3.3.3 validateFeatureChainingFeaturesNotOne; pilot validateFeatureChainingFeatureNotOne, FeatureChain_invalid.kerml.xt).
package P05SingleChainingFeature {
    class C {
        feature a : C;
    }
    feature c chains C::a;
}
