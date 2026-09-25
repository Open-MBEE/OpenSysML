// Invalid: The subject of a case usage must be its first parameter (SysML 7.21 Cases; pilot validateCaseUsageSubjectParameterPosition).
package S13CaseSubjectNotFirst {
    part def P;
    case c {
        in x : P;
        subject s : P;
    }
}
