// A substate completing into its composite's `done` completes the composite,
// whose completion transition leads on; only the machine's own `done` ends the
// run. Every completion is dispatched by a step of its own, so the behaviors
// run in order: s1 in, s11 in, s11 out, s1 out, s2 in, s2 out.
package test {
    state sm {
        attribute log : String = "";

        entry; then s1;
        state s1 {
            entry action { assign log := log + "s1;"; }
            exit action { assign log := log + "/s1;"; }
            entry; then s11;
            state s11 {
                entry action { assign log := log + "s11;"; }
                exit action { assign log := log + "/s11;"; }
            }
            transition first s11 then done;
        }
        state s2 {
            entry action { assign log := log + "s2;"; }
            exit action { assign log := log + "/s2;"; }
        }
        transition first s1 then s2;
        transition first s2 then done;
    }
}
