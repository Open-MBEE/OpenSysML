// Invalid: Only one subject is allowed (SysML 7.20 validateRequirementDefinitionOnlyOneSubject; pilot RequirementSubject_Invalid.sysml.xt).
package P14RequirementTwoSubjects {
    part def B;
    requirement def R {
        subject s1 : B;
        subject s2 : B;
    }
}
