package test {
    private import ScalarValues::*;

    // A behavior written as an action usage with neither a body nor an action
    // performed (`entry action hello;`) names an action of no content: it
    // executes as nothing, so the machine passes through and the assignment
    // beside it still runs.
    item def Go;

    state TestActionOfNoContent {
        attribute counter : Integer = 0;

        entry; then idle;
        state idle {
            entry action hello;
            do action log;
            exit action bye;
        }
        state work {
            entry action tick {
                assign counter := counter + 1;
            }
        }

        transition first idle accept Go
            do action note
            then work;
    }
}
