package Test {
    attribute def Go;

    state Machine {
        attribute log : String = "";

        entry; then idle;
        state idle;
        state working parallel {
            state left {
                entry action boot { }
                transition boot do action {
                    assign log := log + "a-effect ";
                } then active;
                state active {
                    entry action { assign log := log + "a-entry "; }
                }
            }
            state right {
                entry action boot { }
                transition boot do action {
                    assign log := log + "b-effect ";
                } then active;
                state active {
                    entry action { assign log := log + "b-entry "; }
                }
            }
        }

        transition first idle accept Go then working;
    }
}
