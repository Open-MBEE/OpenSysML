// Invalid: a conjugated type cannot be the specific type of a specialization (KerML 8.3.3.1 validateSpecializationSpecificNotConjugated; pilot validateSpecializationSpecificNotConjugated).
package K08ConjugatedTypeSpecialized {
    class A;
    class B ~ A;
    specialization S subtype B specializes A;
}
