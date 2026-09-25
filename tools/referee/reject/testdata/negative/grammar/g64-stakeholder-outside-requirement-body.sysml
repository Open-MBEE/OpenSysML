// Invalid: StakeholderMember is a RequirementBodyItem; a part def body admits no 'stakeholder' (SysML.xtext:2098 StakeholderMember, :2039 RequirementBodyItem; pilot validateStakeholderMembershipOwningType).
package G64StakeholderOutsideRequirementBody {
    part def Person;
    part def System {
        stakeholder owner : Person;
    }
}
