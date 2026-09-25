// A transition inside an orthogonal region reaching a composite state enters the
// state that state's entry transition chooses, and the region's active state is
// that nested state: its outgoing transitions fire and its timer runs.
package test {
    private import ScalarValues::*;

    attribute def Start;
    attribute def Go;

    state sm parallel {
        attribute log : String = "";

        state work {
            entry; then idle;
            state idle;
            transition first idle accept Start then running;
            state running {
                entry action { assign log := log + "running;"; }
                then waiting;
                state waiting {
                    entry action { assign log := log + "waiting;"; }
                }
                transition first waiting accept Go then finished;
                state finished {
                    entry action { assign log := log + "finished;"; }
                }
            }
        }
        state monitor {
            entry; then watching;
            state watching;
        }
    }
}
