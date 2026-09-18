// Invalid: At most one individual definition is allowed (SysML 7.9 validateOccurrenceUsageIndividualDefinition; pilot IndividualUsage_Invalid.sysml.xt:36).
package P25TwoIndividualDefinitions {
    part def A;
    part def B;
    individual def A_1 :> A;
    individual def B_1 :> B;
    individual two_types : A_1, B_1;
}
