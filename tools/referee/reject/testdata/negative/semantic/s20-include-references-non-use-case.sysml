// Invalid: An include use case usage must reference a use case usage (SysML 7.24 Use Cases; pilot validateIncludeUseCaseUsageReference).
package S20IncludeReferencesNonUseCase {
    use case def UC;
    ref r : UC;
    use case def UC2 {
        include r;
    }
}
