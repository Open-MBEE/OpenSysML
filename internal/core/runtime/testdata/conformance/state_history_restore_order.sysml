// Oracle (docs/project/behavior-semantic-oracle.md): `Go` moves both regions of
// `work` on, `Pause` leaves `work`, and `Resume` restores it through its deep
// history: `l1` and `r1` are entered again, and which is entered first is as
// open as it was when they were first reached.
package HistoryRestoreOrder {
    private import ScalarValues::*;

    state Machine {
        attribute log : String = "";

        entry; then work;
        state work parallel {
            state left {
                entry; then l0;
                state l0;
                state l1 { entry { assign log := log + "l1(entry) "; } }
                transition first l0 accept Go then l1;
            }
            state right {
                entry; then r0;
                state r0;
                state r1 { entry { assign log := log + "r1(entry) "; } }
                transition first r0 accept Go then r1;
            }
            deep history resume;
        }
        state paused { entry { assign log := log + "paused(entry) "; } }

        transition first work accept Pause then paused;
        transition first paused accept Resume then resume;
        transition first resume then work;
    }
}
