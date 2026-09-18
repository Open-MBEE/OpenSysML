// Invalid: A requirement usage inherits a subject from its definition and another from the requirement it reference-subsets, so it has two (SysML 7.20 Requirements; pilot validateRequirementUsageOnlyOneSubject).
package S92RequirementReferencesSecondSubject {
    requirement def A { subject a; }
    requirement b { subject b; }
    requirement c : A ::> b;
}
