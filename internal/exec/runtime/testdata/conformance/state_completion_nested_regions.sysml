// The orthogonal regions of a composite state complete the state, not the
// machine: `busy` stays active while one of its regions still runs, and once
// both reach `done` its own completion transition fires, exiting `busy` and
// completing the machine through its `done`.
package test {
    attribute def First;
    attribute def Second;

    state sm {
        attribute log : String = "";

        entry; then busy;
        state busy parallel {
            exit action { assign log := log + "busy;"; }
            state left {
                entry; then lstart;
                state lstart {
                    exit action { assign log := log + "l;"; }
                }
                transition first lstart accept First then done;
            }
            state right {
                entry; then rstart;
                state rstart {
                    exit action { assign log := log + "r;"; }
                }
                transition first rstart accept Second then done;
            }
        }
        transition first busy then done;
    }
}
