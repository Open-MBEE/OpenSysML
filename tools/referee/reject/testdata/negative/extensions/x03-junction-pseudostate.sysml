// Invalid: 'junction' is an OpenSysML invention; the pinned StateBodyItem has no pseudostates (SysML.xtext StateBodyItem).
package X03JunctionPseudostate {
    state def S {
        state receiving;
        junction route;
    }
}
