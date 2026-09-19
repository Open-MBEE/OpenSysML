package Test {
    private import ScalarValues::*;

    attribute def Stop;

    // A state's inline do body is interrupted where a transition out of the state
    // is triggered: the body yields after each statement, the Stop queued is
    // dispatched after its first, and the transition drops the two statements
    // still to run with the behavior — the exit behavior runs, s2 and s3 never do.
    state def Machine {
        attribute log : String = "";

        entry; then busy;
        state busy {
            do action work {
                assign log := log + "s1 ";
                assign log := log + "s2 ";
                assign log := log + "s3 ";
            }
            exit action leave {
                assign log := log + "exit ";
            }
        }
        state halted;

        transition first busy accept Stop then halted;
    }
}
