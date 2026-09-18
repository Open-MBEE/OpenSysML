// Invalid: the featuring type of a type featuring must be a type, not a package (KerML 8.3.4.3 TypeFeaturing::featuringType is typed Type; pilot fails to link the TypeFeaturing — `Couldn't resolve reference to Type 'Q'`).
package K73FeaturingTypeIsPackage {
    package Q;
    feature f;
    featuring f by Q;
}
