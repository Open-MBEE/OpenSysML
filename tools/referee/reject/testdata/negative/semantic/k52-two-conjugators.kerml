// Invalid: a type may have at most one conjugator (KerML 8.3.3.1 validateTypeAtMostOneConjugator; pilot validateTypeAtMostOneConjugator).
package K45TwoConjugators {
    classifier A;
    classifier B;
    classifier C ~A ~B;
}
