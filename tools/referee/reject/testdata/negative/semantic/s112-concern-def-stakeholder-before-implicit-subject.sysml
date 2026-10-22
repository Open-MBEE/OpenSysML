// Invalid: A stakeholder declared before any subject is the first input of a concern definition, displacing the subject it inherits (SysML 8.3.21.8 validateRequirementDefinitionSubjectParameterPosition; pilot validateRequirementDefinitionSubjectParameterPosition).
package S112ConcernDefStakeholderBeforeImplicitSubject {
    part def P;
    concern def C {
        stakeholder owner : P;
    }
}
