// Invalid: An owned usage of a variation usage must be a variant, an objective included (SysML 8.3.6.4 validateUsageVariationOwnedFeatureMembership; pilot validateUsageVariationMembership).
package VariationUsageObjectiveNotVariant {
    requirement def R;
    variation case choices {
        objective o : R;
        variant case a;
    }
}
