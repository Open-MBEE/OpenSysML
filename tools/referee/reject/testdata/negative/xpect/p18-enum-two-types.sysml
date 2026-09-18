// Invalid: An enumeration must be typed by one enumeration definition (SysML 7.7 validateEnumerationUsageType; pilot EnumerationUsage_invalid.sysml.xt).
package P18EnumTwoTypes {
    enum def E { e1; }
    enum def EE { ee1; }
    enum e3 : E, EE;
}
