// Invalid: A stakeholder a framed concern declares precedes the subject it inherits from its definition (SysML 8.3.21.9 validateRequirementUsageSubjectParameterPosition; pilot validateRequirementUsageSubjectParameterPosition).
package S113FramedConcernStakeholderBeforeInheritedSubject {
    part def P;
    concern def C {
        subject s : P;
        stakeholder owner : P;
    }
    viewpoint def V {
        subject s : P;
        frame concern c : C {
            stakeholder reviewer : P;
        }
    }
}
