// Invalid: the conjugated type of a keyword-first conjugation must be a type, not a package (KerML 8.3.3.4 Conjugation::conjugatedType is typed Type; pilot fails to link the Conjugation — `Couldn't resolve reference to Type 'Q'`).
package K66ConjugatedTypeIsPackage {
    package Q;
    class B;
    conjugate Q ~ B;
}
