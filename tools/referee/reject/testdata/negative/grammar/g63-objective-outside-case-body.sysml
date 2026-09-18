// Invalid: ObjectiveMember is a CaseBodyItem; a part def body admits no 'objective' (SysML.xtext:2198 ObjectiveMember, :2190 CaseBodyItem; pilot validateObjectiveMembershipOwningType).
package G63ObjectiveOutsideCaseBody {
    requirement def Goal;
    part def System {
        objective o : Goal;
    }
}
