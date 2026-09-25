// An entry transition may choose `done`: the machine then completes as it starts,
// running its own exit behavior, so a guard can make a machine finish at once.
package test {
    private import ScalarValues::*;

    state sm {
        attribute skip : Boolean = true;
        attribute log : String = "";

        entry action { assign log := log + "in;"; }
        if skip then done;
        then working;
        exit action { assign log := log + "out;"; }
        state working {
            entry action { assign log := log + "working;"; }
        }
    }
}
