// Invalid: A case usage may have at most one subject (SysML 7.21 Cases; pilot validateCaseUsageOnlyOneSubject).
package S12CaseTwoSubjects {
    part def P;
    case c {
        subject s1 : P;
        subject s2 : P;
    }
}
