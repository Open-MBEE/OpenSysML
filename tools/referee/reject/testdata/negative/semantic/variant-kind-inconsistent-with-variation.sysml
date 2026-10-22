// Invalid: the kind of a variant usage must be consistent with the kind of its owning variation (SysML 7.6.7; checkUsageVariationDefinitionSpecialization, checkUsageVariationUsageSpecialization; pilot SysMLValidator checkActionUsage typing).
package VariantKindInconsistentWithVariation {
    part def P;
    action def A;
    variation part def PartChoices :> P {
        variant action a1;
        variant part p1 : P;
    }
    variation part engine : P {
        variant action a2 : A;
        variant attribute size;
        variant part p2 : P;
    }
    variation attribute def SizeChoices {
        variant snapshot part s1;
    }
}
