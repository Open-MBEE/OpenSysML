// A composite state whose regions all reached `done` but which has no
// completion transition stays completed and active: the machine keeps running
// with `busy` at `done+done`, and `busy` still takes the event it accepts.
package test {
    attribute def First;
    attribute def Second;
    attribute def Reset;

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
        state idle;
        transition first busy accept Reset then idle;
    }
}
