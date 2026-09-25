// Invalid: the inverting feature of a keyword-first feature inverting must be a feature, not a class (KerML 8.3.4.8 FeatureInverting::invertingFeature is typed Feature; pilot fails to link the FeatureInverting — `Couldn't resolve reference to Feature 'C'`).
package K69InvertingFeatureIsClass {
    class C;
    feature f;
    inverting inverse f of C;
}
