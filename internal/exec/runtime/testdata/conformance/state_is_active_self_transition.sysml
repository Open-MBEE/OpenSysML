package test {
    private import ScalarValues::*;
    private import StateActivity::*;

    // A self-transition exits and re-enters its state: the state reads
    // inactive while the transition's effect runs and active again in its
    // entry, which runs once more.
    item def Again;

    state def Loop {
        attribute entries : Integer = 0;
        attribute activeInEntry : Boolean = false;
        attribute activeInExit : Boolean = false;
        attribute activeInEffect : Boolean = true;

        entry; then fill;
        state fill {
            entry action {
                assign entries := entries + 1;
                assign activeInEntry := fill.isActive;
            }
            exit assign activeInExit := fill.isActive;
        }
        transition t1
            first fill
            accept Again
            do assign activeInEffect := fill.isActive
            then fill;
    }
}
