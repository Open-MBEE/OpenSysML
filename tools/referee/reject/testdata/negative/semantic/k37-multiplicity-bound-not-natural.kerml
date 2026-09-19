// Invalid: a multiplicity bound must have a Natural value (KerML 8.3.3.6 validateMultiplicityRangeResultTypes; pilot validateMultiplicityRangeResultTypes).
package K37MultiplicityBoundNotNatural {
    class C {
        feature n : C;
        feature f : C [n];
    }
}
