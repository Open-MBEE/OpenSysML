// Invalid: A variation must not specialize another variation (SysML 7.9 validateVariationSpecialization; pilot Variability_invalid.sysml.xt:50).
package P10VariationSpecializesVariation {
    variation attribute def AttributeChoices {
        variant attribute a1;
    }
    variation attribute def AttributeChoices1 :> AttributeChoices;
}
