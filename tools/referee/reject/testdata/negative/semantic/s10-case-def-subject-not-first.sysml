// Invalid: The subject of a case definition must be its first parameter (SysML 7.21 Cases; pilot validateCaseDefinitionSubjectParameterPosition).
package S10CaseDefSubjectNotFirst {
    part def P;
    case def CD {
        in x : P;
        subject s : P;
    }
}
