// Invalid: An include use case usage must reference a use case usage; a use case definition is not a feature (SysML 7.24 Use Cases; pilot validateIncludeUseCaseUsageReference).
package S20bIncludeReferencesDefinition {
    use case def UD;
    use case uc1 : UD;
    use case def Outer {
        include UD;
        include uc1;
    }
}
