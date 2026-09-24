// Invalid: 'initial' is an OpenSysML invention; the pinned StateBodyItem has no such member (SysML.xtext StateBodyItem).
package X01InitialStateMarker {
    state def S {
        initial start;
        state idle;
        transition start then idle;
    }
}
