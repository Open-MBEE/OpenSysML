// Invalid: a package-level feature has no featuring type, so a feature of a type is not accessible to it (KerML 8.3.3.3 validateSubsettingFeaturingTypes; pilot validateSubsettingFeaturingTypes).
package S105PackageLevelRedefinesTypeFeature {
    part def Engine;
    part def Car {
        part engine : Engine;
    }
    part e2 : Engine :>> Car::engine;
}
