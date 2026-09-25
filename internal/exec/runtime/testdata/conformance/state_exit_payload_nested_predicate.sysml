// A constraint a state declares reads the payload of the transition leaving it,
// `raise.l`, when the state's exit action applies it: the predicate closes over
// the state's performance, which is read within the firing.
package StateExitPayloadNestedPredicate {
    private import ScalarValues::*;

    attribute def Level :> Integer;

    state Monitor {
        attribute high : Boolean = false;
        attribute exited : Integer = 0;

        entry; then idle;
        state idle {
            constraint aboveFive { raise.l > 5 }
            exit action {
                in level : Integer = raise.l;
                assign high := aboveFive();
                assign exited := level;
            }
        }
        state alerting;

        transition raise first idle accept l : Level then alerting;
        transition first alerting then done;
    }
}
