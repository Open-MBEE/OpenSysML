// Invalid: a crossed feature must chain through the opposite end feature (KerML 8.3.3.3 validateCrossSubsettingCrossedFeature; pilot validateCrossSubsettingCrossedFeature).
package K17CrossesNotThroughOppositeEnd {
    class A;
    assoc S {
        end a : A;
        end b : A crosses a;
    }
}
