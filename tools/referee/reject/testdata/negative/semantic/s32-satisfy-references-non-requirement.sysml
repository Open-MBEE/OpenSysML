// Invalid: A satisfy requirement usage must reference a requirement usage (SysML 7.20 Requirements; pilot validateSatisfyRequirementUsageReference).
package S32SatisfyReferencesNonRequirement {
    constraint c;
    part p {
        satisfy c;
    }
}
