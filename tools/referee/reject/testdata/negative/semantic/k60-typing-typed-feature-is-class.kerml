// Invalid: the typed feature of a keyword-first feature typing must be a feature, not a class (KerML 8.3.4.4 FeatureTyping::typedFeature is typed Feature; pilot fails to link the FeatureTyping — `Couldn't resolve reference to Feature 'C'`).
package K60TypedFeatureIsClass {
    class C;
    class B;
    typing C typed by B;
}
