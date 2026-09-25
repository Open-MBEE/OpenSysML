// Invalid: RequirementConstraintMember requires a constraint usage after the require keyword (SysML.xtext:2057 RequirementConstraintMember).
package G18RequireWithoutConstraint {
    requirement def R {
        require ;
    }
}
