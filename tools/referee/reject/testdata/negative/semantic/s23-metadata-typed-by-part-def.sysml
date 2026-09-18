// Invalid: A metadata usage must be typed by one metadata definition (SysML 7.26 Metadata; pilot validateMetadataUsageType).
package S23MetadataTypedByPartDef {
    part def PD;
    part p {
        @PD;
    }
}
