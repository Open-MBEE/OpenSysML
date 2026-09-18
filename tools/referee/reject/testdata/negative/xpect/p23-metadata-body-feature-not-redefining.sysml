// Invalid: A metadata body feature must redefine an owning-type feature (SysML 7.24 checkMetadataFeatureRedefinition; pilot MetadataUsage_Invalid.sysml.xt:93).
package P23MetadataBodyFeatureNotRedefining {
    metadata def A {
        attribute x : ScalarValues::Integer;
    }
    attribute bad;
    attribute a {
        @A {
            bad;
        }
    }
}
