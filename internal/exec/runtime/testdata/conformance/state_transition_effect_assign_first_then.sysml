package test {
    // The standard `first … then` spelling of a transition whose effect is a bare
    // statement (SysML 7.19.4 TransitionUsage effect behavior): the statement is
    // terminated by the transition's next clause, and `counter` ends at 1 only if
    // the effect ran.
    state TestTransitionEffectAssignFirstThen {
        attribute counter : Integer = 0;

        entry; then init;
        state init;
        state active;

        succession first init then active;
        transition first active do assign counter := counter + 1 then done;
    }
}
