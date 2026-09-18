// Invalid: An enumeration attribute cannot have more than one type (SysML 7.8 Enumerations; pilot validateAttributeUsageEnumerationType).
package S06EnumAttributeTwoTypes {
    enum def E { a; b; }
    attribute def D;
    attribute x : E, D;
}
