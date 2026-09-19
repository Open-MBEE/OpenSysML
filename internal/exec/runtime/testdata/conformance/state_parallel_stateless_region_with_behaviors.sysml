// A region of a parallel state stood for by a state declaring entry and exit
// behaviors, but no substates, needs no initial: it starts in that state, runs
// its entry, and runs its exit when the parallel state is left.
package test {
    attribute def Go;
    attribute def Stop;

    state sm {
        attribute log : Integer = 0;

        entry; then busy;
        state busy parallel {
            state work {
                entry; then wstart;
                state wstart;
                transition first wstart accept Go then done;
            }
            state idle {
                entry { assign log := log * 10 + 1; }
                exit { assign log := log * 10 + 2; }
            }
        }
        transition first busy accept Stop then done;
    }
}
