// Invalid: A requirement definition inheriting a subject from each of two generals has two subjects (SysML 7.20 Requirements; pilot validateRequirementDefinitionOnlyOneSubject).
package S90RequirementDefInheritsTwoSubjects {
    requirement def A { subject a; }
    requirement def B { subject b; }
    requirement def C :> A, B;
}
