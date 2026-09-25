// Invalid: the inverted feature of a keyword-first feature inverting must be a feature, not a package (KerML 8.3.4.8 FeatureInverting::featureInverted is typed Feature; pilot fails to link the FeatureInverting — `Couldn't resolve reference to Feature 'Q'`).
package K68FeatureInvertedIsPackage {
    package Q;
    feature f;
    inverse Q of f;
}
