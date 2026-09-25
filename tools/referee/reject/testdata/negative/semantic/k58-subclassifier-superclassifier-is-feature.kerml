// Invalid: the superclassifier of a keyword-first subclassification must be a classifier, not a feature (KerML 8.3.3.2 Subclassification::superclassifier is typed Classifier and 8.3.4 a Feature is not a Classifier; pilot fails to link the Subclassification — `Couldn't resolve reference to Classifier 'f'`).
package K58SuperclassifierIsFeature {
    feature f;
    class B;
    specialization subclassifier B specializes f;
}
