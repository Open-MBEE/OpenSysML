// Invalid: The subject of a requirement usage must be its first parameter (SysML 7.20 Requirements; pilot validateRequirementUsageSubjectParameterPosition).
package S30RequirementSubjectNotFirst {
    part def P;
    requirement r {
        in x : P;
        subject s : P;
    }
}
