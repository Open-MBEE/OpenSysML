// Invalid: A requirement usage may have at most one subject (SysML 7.20 Requirements; pilot validateRequirementUsageOnlyOneSubject).
package S29RequirementTwoSubjects {
    part def P;
    requirement r {
        subject s1 : P;
        subject s2 : P;
    }
}
