// Invalid: 'choice' is an OpenSysML invention; the pinned StateBodyItem has no pseudostates (SysML.xtext StateBodyItem).
package X02ChoicePseudostate {
    state def S {
        state idle;
        choice evaluate;
    }
}
