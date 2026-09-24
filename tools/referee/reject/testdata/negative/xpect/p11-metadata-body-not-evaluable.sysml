// Invalid: A metadata body feature value must be model-level evaluable (SysML 7.24 checkMetadataBodyFeature; pilot MetadataUsage_Invalid.sysml.xt:82).
package P11MetadataBodyNotEvaluable {
    metadata def A {
        attribute x;
    }
    attribute a {
        @A {
            x = ~3;
        }
    }
}
