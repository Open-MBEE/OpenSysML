// Invalid: a feature reference expression must refer to a feature (KerML 8.3.4.8 validateFeatureReferenceExpressionReferentIsFeature; pilot validateFeatureReferenceExpressionReferentIsFeature).
package K39FeatureReferenceToClass {
    class C;
    feature f : C = C;
}
