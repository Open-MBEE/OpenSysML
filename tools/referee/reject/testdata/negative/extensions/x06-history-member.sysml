// Invalid: 'history' is an OpenSysML invention; the pinned StateBodyItem has no history pseudostate (SysML.xtext StateBodyItem).
package X06HistoryMember {
    state def S {
        state idle;
        history resume;
    }
}
