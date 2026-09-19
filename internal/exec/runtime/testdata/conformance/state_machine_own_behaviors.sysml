// A machine's own entry and exit behaviors frame its run whether or not it has
// orthogonal regions: the entry runs before the start state is chosen and
// entered, the exit runs after the state a completing transition leaves.
package test {
    private import ScalarValues::*;

    attribute def Stop;

    state sm {
        attribute log : String = "";

        entry action { assign log := log + "machine-in;"; }
        then working;
        exit action { assign log := log + "machine-out;"; }
        state working {
            entry action { assign log := log + "in;"; }
            exit action { assign log := log + "out;"; }
        }
        transition first working accept Stop then done;
    }
}
