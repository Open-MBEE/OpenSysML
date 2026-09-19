// `done` written inside a composite state completes that state, not the
// machine: `outer`'s substate reaching it fires `outer`'s own completion
// transition, and the machine goes on in `next`.
package test {
    attribute def Go;

    state sm {
        attribute log : String = "";

        entry; then outer;
        state outer {
            exit action { assign log := log + "outer;"; }
            entry; then inner;
            state inner {
                exit action { assign log := log + "inner;"; }
            }
            transition first inner accept Go then done;
        }
        state next {
            entry action { assign log := log + "next;"; }
        }
        transition first outer then next;
    }
}
