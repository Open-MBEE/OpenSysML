// Invalid: 'defer' is an OpenSysML invention; the pinned StateBodyItem has no event deferral (SysML.xtext StateBodyItem).
package X05DeferMember {
    state def S {
        state idle {
            defer Ping;
        }
    }
}
