// Invalid: a package-level feature has no featuring type, so a feature featured by a type is not accessible to it (KerML 8.3.3.3 validateSubsettingFeaturingTypes; pilot validateSubsettingFeaturingTypes).
package K74PackageLevelRedefinesFeaturedFeature {
    class Engine;
    feature f : Engine featured by Engine;
    feature g : Engine :>> f;
}
