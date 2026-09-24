// Invalid: EntryActionMember is a StateBodyItem; a part def body admits no 'entry action' (SysML.xtext:1772 EntryActionMember; pilot validateStateSubactionMembershioOwningType).
package G65EntryActionOutsideStateBody {
    part def System {
        entry action init;
    }
}
