// Invalid: An assignment must have a feature as its referent (SysML 7.16 Actions; pilot validateAssignmentActionUsageReferent).
package S43AssignToNonFeature {
    part def PD;
    action a {
        assign PD := 1;
    }
}
