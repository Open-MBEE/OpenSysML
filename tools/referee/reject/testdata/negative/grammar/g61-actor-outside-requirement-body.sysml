// Invalid: ActorMember is a RequirementBodyItem/CaseBodyItem; a part def body admits no 'actor' (SysML.xtext:2089 ActorMember, :2039 RequirementBodyItem; pilot validateActorMembershipOwningType).
package G61ActorOutsideRequirementBody {
    part def Person;
    part def System {
        actor operator : Person;
    }
}
