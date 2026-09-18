// Invalid: An owned usage of a variation usage must be a variant (SysML 7.6 Definition and Usage; pilot validateUsageVariationMembership).
package S37VariationUsageMemberNotVariant {
    part def P;
    variation part choices : P {
        variant part a : P;
        part b : P;
    }
}
