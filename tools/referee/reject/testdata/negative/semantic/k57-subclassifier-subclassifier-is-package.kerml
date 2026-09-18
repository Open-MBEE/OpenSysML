// Invalid: the subclassifier of a keyword-first subclassification must be a classifier, not a package (KerML 8.3.3.2 Subclassification::subclassifier is typed Classifier; pilot fails to link the Subclassification — `Couldn't resolve reference to Classifier 'Q'`).
package K57SubclassifierIsPackage {
    package Q;
    class B;
    subclassifier Q :> B;
}
