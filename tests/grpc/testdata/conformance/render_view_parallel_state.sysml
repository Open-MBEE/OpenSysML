package ParallelDemo {
    private import StandardViewDefinitions::*;
    // A parallel state's substates run at once; its node says «parallel».
    state def Operating {
        entry; then running;
        state running parallel {
            state heating;
            state logging;
        }
        state off;
    }
    view operating : StateTransitionView {
        expose Operating;
    }
}
