// A region without an entry transition remains inactive, including when its
// only state declares entry and exit behaviors.
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
