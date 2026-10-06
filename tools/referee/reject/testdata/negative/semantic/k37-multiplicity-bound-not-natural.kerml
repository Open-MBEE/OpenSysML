// Invalid: a multiplicity bound must have a Natural value (KerML 8.3.4.11.2 validateMultiplicityRangeResultTypes; pilot validateMultiplicityRangeResultTypes).
package K37MultiplicityBoundNotNatural {
    class C {
        feature n : C;
        feature f : C [n];
    }
}
