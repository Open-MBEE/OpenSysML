// Invalid: An enumeration definition is a variation and a variation must not specialize another variation (SysML v2 7.6.4 and 8.3.9 validateEnumerationDefinitionIsVariation, validateDefinitionVariationSpecialization; pilot validateDefinitionVariationSpecialization).
package S01EnumerationSpecializesEnumeration {
    enum def E {
        a;
        b;
    }
    enum def F :> E;
}
