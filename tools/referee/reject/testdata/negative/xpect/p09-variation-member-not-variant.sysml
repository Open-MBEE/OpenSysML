// Invalid: An owned usage of a variation must be a variant (SysML 7.9 validateDefinitionVariationMembership; pilot Variability_invalid.sysml.xt:46).
package P09VariationMemberNotVariant {
    variation attribute def AttributeChoices {
        variant attribute a1;
        attribute a2;
    }
}
