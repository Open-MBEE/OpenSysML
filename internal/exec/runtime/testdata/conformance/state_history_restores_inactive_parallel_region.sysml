package HistoryInactiveRegion {
    item def Leave;
    item def ReturnShallow;
    item def ReturnDeep;

    state Machine {
        attribute log : String = "";
        entry; then working;
        state working parallel {
            state a {
                entry; then active;
                state active {
                    entry { assign log := log + "A(entry) "; }
                    exit { assign log := log + "A(exit) "; }
                }
            }
            state b {
                state inactive {
                    entry { assign log := log + "B(entry) "; }
                    exit { assign log := log + "B(exit) "; }
                }
            }
            history shallow;
            deep history deep;
        }
        state outside;
        state completed;

        transition first active when Leave then outside;
        transition first outside when ReturnShallow then shallow;
        transition first outside when ReturnDeep then deep;
        transition first working then completed;
    }
}
