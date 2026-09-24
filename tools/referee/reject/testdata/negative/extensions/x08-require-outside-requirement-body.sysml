// Invalid: Only requirements can have assumed or required constraints; 'require' in a part def body is an OpenSysML extension (SysML 7.20 Requirements; pilot validateRequirementConstraintMembershipOwningType, enforced by SysML.xtext RequirementBodyItem).
package X08RequireOutsideRequirementBody {
    part def System {
        require constraint { true }
    }
}
