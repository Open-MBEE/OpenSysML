// A region of a parallel machine stood for by a state with no substates needs
// no initial, as one of a parallel state does not: it starts in that state and
// stays there while its sibling region runs to done.
package test {
    attribute def Go;

    state def Machine parallel {
        state work {
            entry; then wstart;
            state wstart;
            transition first wstart accept Go then done;
        }
        state idle;
    }
}
