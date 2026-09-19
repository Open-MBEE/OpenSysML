// Invalid: the featured feature of a type featuring must be a feature, not a class (KerML 8.3.4.3 TypeFeaturing::featureOfType is typed Feature; pilot fails to link the TypeFeaturing — `Couldn't resolve reference to Feature 'C'`).
package K72FeatureOfTypeIsClass {
    class C;
    class B;
    featuring C by B;
}
