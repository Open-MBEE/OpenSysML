// Invalid: An assignment referent must be time varying (SysML 7.16 Actions; pilot validateAssignmentActionUsageReferentIsTimeVarying).
package S05AssignToPackageLevelAttribute {
    attribute x : ScalarValues::Integer;
    action a {
        assign x := 1;
    }
}
