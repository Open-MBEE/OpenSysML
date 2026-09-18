// Invalid: the subsetting feature of a keyword-first subsetting must be a feature, not a package (KerML 8.3.4.5 Subsetting::subsettingFeature is typed Feature; pilot fails to link the Subsetting — `Couldn't resolve reference to Feature 'Q'`).
package K62SubsettingFeatureIsPackage {
    package Q;
    feature f;
    subset Q :> f;
}
