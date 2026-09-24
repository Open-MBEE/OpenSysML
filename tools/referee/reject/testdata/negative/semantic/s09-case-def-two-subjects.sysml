// Invalid: A case definition may have at most one subject (SysML 7.21 Cases; pilot validateCaseDefinitionOnlyOneSubject).
package S09CaseDefTwoSubjects {
    part def P;
    case def CD {
        subject s1 : P;
        subject s2 : P;
    }
}
