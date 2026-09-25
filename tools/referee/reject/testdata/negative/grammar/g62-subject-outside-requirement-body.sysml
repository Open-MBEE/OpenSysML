// Invalid: SubjectMember is a RequirementBodyItem/CaseBodyItem; a part def body admits no 'subject' (SysML.xtext:2049 SubjectMember, :2039 RequirementBodyItem; pilot validateSubjectMembershipOwningType).
package G62SubjectOutsideRequirementBody {
    part def Vehicle;
    part def System {
        subject v : Vehicle;
    }
}
