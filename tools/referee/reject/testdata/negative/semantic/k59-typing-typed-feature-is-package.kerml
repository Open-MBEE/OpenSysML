// Invalid: the typed feature of a keyword-first feature typing must be a feature, not a package (KerML 8.3.4.4 FeatureTyping::typedFeature is typed Feature; pilot fails to link the FeatureTyping — `Couldn't resolve reference to Feature 'Q'`).
package K59TypedFeatureIsPackage {
    package Q;
    class B;
    typing Q : B;
}
