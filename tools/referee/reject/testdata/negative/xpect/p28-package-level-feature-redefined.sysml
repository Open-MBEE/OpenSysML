// Invalid: A package-level feature cannot be redefined (KerML 7.4.7 validateRedefinitionFeaturingTypes; pilot Redefinition_OwningType_Invalid.sysml.xt:28).
package P28PackageLevelFeatureRedefined {
    part def Wheel;
    part wheel : Wheel;
    part wheel1 redefines wheel;
}
