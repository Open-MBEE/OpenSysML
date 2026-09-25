// Invalid: An enumeration must be typed by one enumeration definition — an expression body written as the value types the enumerated value by Evaluation, which the enumeration is not (SysML 7.7 validateEnumerationUsageType; pilot SysMLValidator checkOneType).
package S51EnumValueBodyTypedOutsideEnumeration {
    enum def E {
        a = { 1 + 2 };
    }
}
