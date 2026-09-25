// A transition reaching `done` completes the machine: the state it leaves runs
// its exit action first.
package test {
    private import ScalarValues::*;

    attribute def Stop;

    state sm {
        attribute log : String = "";

        entry; then start;
        state start;
        state working {
            entry action { assign log := log + "in;"; }
            exit action { assign log := log + "out;"; }
        }
        succession first start then working;
        transition first working accept Stop then done;
    }
}
