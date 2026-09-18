// Invalid: an enumeration body admits only enumerated values and annotating elements, not a nested definition (SysML.xtext EnumerationBody).
package G61DefinitionInEnumerationBody {
    enum def Level {
        attribute def Nested;
        low;
        high;
    }
}
