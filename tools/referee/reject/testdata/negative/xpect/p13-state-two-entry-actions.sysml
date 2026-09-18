// Invalid: A state may have at most one entry action (SysML 7.16 validateStateDefinitionSubactionKind; pilot StateSubactions_invalid.sysml.xt).
package P13StateTwoEntryActions {
    state def S {
        entry action a1;
        entry action a2;
    }
}
