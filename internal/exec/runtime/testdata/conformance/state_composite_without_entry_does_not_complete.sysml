package CompositeWithoutEntry {
    item def Go;
    item def Back;
    item def Direct;
    item def Finish;

    state Machine {
        attribute log : String = "";
        entry; then idle;
        state idle;
        state Work {
            state W1 { entry { assign log := log + "W1(entry) "; } }
            transition first W1 accept Finish then done;
        }
        state X { entry { assign log := log + "X(entry) "; } }
        transition first idle accept Go then Work;
        transition first Work accept Back then idle;
        transition first idle accept Direct then W1;
        transition first Work then X;
    }
}
