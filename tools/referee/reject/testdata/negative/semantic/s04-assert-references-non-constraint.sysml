// Invalid: An assert constraint usage must reference a constraint usage (SysML 7.19 Constraints; pilot validateAssertConstraintUsageReference).
package S04AssertReferencesNonConstraint {
    constraint def C;
    ref r : C;
    part p {
        assert r;
    }
}
