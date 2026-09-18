// Invalid: the original type of a keyword-first conjugation must be a type, not a package (KerML 8.3.3.4 Conjugation::originalType is typed Type; pilot fails to link the Conjugation — `Couldn't resolve reference to Type 'Q'`).
package K67OriginalTypeIsPackage {
    package Q;
    class B;
    conjugation conjugate B conjugates Q;
}
