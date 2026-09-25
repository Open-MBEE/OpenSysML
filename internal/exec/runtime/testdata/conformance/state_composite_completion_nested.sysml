// Completion climbs one composite at a time: the leaf completes `inner`, whose
// completion transition reaches `outer`'s `done` and completes `outer`, whose
// completion transitions are then scheduled with their guards read on `outer`'s
// data, the one that holds leading into `next`.
package test {
    private import ScalarValues::*;
    attribute def Go;

    state sm {
        attribute log : String = "";
        attribute rounds : Integer = 0;

        entry; then outer;
        state outer {
            exit action { assign log := log + "/outer;"; }
            entry; then inner;
            state inner {
                exit action { assign log := log + "/inner;"; }
                entry; then leaf;
                state leaf {
                    exit action { assign log := log + "/leaf;"; }
                }
                transition first leaf accept Go do assign rounds := rounds + 1 then done;
            }
            transition first inner then done;
        }
        state next {
            entry action { assign log := log + "next;"; }
        }
        state other {
            entry action { assign log := log + "other;"; }
        }
        transition first outer if rounds == 1 then next;
        transition first outer if rounds == 0 then other;
    }
}
