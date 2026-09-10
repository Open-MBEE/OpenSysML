package Test {
    // A state may be written as a redefinition of the one it overrides
    // (`state redefines waiting`), answering to the name of what it redefines
    // (KerML 7.3.4.5), so a sourceless accept inside it and a transition naming
    // it must both still find it.
    state Machine : Base {
        entry; then init;
        state init;
        state redefines waiting;
        accept go then active;
        state active;

        succession first init then waiting;
        succession first active then done;
    }

    state def Base {
        state waiting;
    }
}
