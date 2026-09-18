// Invalid: The subject of a requirement definition must be its first parameter (SysML 7.20 Requirements; pilot validateRequirementDefinitionSubjectParameterPosition).
package S28RequirementDefSubjectNotFirst {
    part def P;
    requirement def R {
        in x : P;
        subject s : P;
    }
}
