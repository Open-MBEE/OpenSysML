// Invalid: 'defer' was an OpenSysML extension, since removed; the pinned StateBodyItem has no event deferral (SysML.xtext StateBodyItem).
package X05DeferMember {
    state def S {
        state idle {
            defer Ping;
        }
    }
}
