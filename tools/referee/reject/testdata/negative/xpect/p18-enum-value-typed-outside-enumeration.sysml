// Invalid: An enumeration must be typed by one enumeration definition — a literal value types the enumerated value by Integer, which the enumeration is not (SysML 7.7 validateEnumerationUsageType; pilot SysMLValidator checkOneType).
package P18EnumValueTypedOutsideEnumeration {
    private import ScalarValues::Real;
    enum def E :> Real {
        a = 4.0;
        b = 3;
    }
}
