package InactiveParallelRegion {
    item def Go;
    item def Back;
    item def Direct;
    item def FinishA;
    item def FinishB;

    state Machine {
        attribute log : String = "";
        entry; then idle;
        state idle;
        state working parallel {
            state a {
                entry; then A;
                state A { entry { assign log := log + "A(entry) "; } }
                transition first A accept FinishA then done;
            }
            state b {
                state B { entry { assign log := log + "B(entry) "; } }
                transition first B accept FinishB then done;
            }
        }
        state outside { entry { assign log := log + "outside(entry) "; } }
        transition first idle accept Go then working;
        transition first idle accept Direct then B;
        transition first working accept Back then idle;
        transition first working then outside;
    }
}
