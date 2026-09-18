// Invalid: the subsetted feature of a keyword-first subsetting must be a feature, not a class (KerML 8.3.4.5 Subsetting::subsettedFeature is typed Feature; pilot fails to link the Subsetting — `Couldn't resolve reference to Feature 'C'`).
package K63SubsettedFeatureIsClass {
    class C;
    feature f;
    specialization subset f subsets C;
}
