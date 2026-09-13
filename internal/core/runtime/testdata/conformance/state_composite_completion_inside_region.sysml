// A composite state nested in an orthogonal region completes that region
// through its own completion transition: `inner` reaching `done` fires
// `inner then done`, which completes `left`; with `right` already done, `outer`
// completes and its completion transition enters `next`.
package test {
    attribute def Go;

    state sm {
        attribute log : String = "";

        entry; then outer;
        state outer parallel {
            exit action { assign log := log + "/outer;"; }
            state left {
                entry; then inner;
                state inner {
                    exit action { assign log := log + "/inner;"; }
                    entry; then wait;
                    state wait;
                    transition first wait accept Go then done;
                }
                transition first inner then done;
            }
            state right {
                entry; then done;
            }
        }
        state next {
            entry action { assign log := log + "next;"; }
        }
        transition first outer then next;
    }
}
