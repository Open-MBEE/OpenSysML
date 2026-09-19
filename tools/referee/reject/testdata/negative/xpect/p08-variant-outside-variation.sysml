// Invalid: A variant must be an owned member of a variation (SysML 7.9 validateVariationMembershipOwningNamespace; pilot Variability_invalid.sysml.xt:55).
package P08VariantOutsideVariation {
    part def B;
    part b1 : B;
    part def PartChoices :> B {
        variant b1;
    }
}
