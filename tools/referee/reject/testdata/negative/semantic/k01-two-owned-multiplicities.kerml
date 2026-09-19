// Invalid: a type may own at most one multiplicity (KerML 8.3.3.1 validateTypeOwnedMultiplicity; pilot validateTypeOwnedMultiplicity).
package K01TwoOwnedMultiplicities {
    class C [1] {
        multiplicity m [2];
    }
}
