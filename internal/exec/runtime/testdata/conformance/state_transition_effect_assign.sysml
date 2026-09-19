package test {
    // A transition's effect may be written as a bare statement (SysML 7.19.4
    // TransitionUsage effect behavior), terminated by the ';' that terminates the
    // transition: the compact `to` spelling takes exactly one ';', and `counter`
    // ends at 1 only if the effect ran.
    state TestTransitionEffectAssign {
        attribute counter : Integer = 0;

        entry; then init;
        state init;
        state active;

        succession first init then active;
        transition first active do assign counter := counter + 1 then done;
    }
}
