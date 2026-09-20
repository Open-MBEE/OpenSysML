// A region of a parallel state stood for by a state with no substates needs no
// initial: it starts in that state and stays there while its sibling region runs.
package test {
    attribute def Go;
    attribute def Stop;

    state sm {
        entry; then busy;
        state busy parallel {
            state work {
                entry; then wstart;
                state wstart;
                state wdone;
                transition first wstart accept Go then wdone;
            }
            state idle;
        }
        transition first busy accept Stop then done;
    }
}
