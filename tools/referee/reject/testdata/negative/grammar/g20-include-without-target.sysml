// Invalid: IncludeUseCaseUsage requires a referenced use case or a use case declaration after include (SysML.xtext:2300 IncludeUseCaseUsage).
package G20IncludeWithoutTarget {
    use case def U {
        include ;
    }
}
