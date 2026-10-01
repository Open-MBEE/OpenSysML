// One exit action is performed by each transition leaving its state, and reads
// the payload of the one being taken: a transition not taken carries nothing.
package StateExitSharedByTwoTransitions {
    private import ScalarValues::*;

    attribute def Warning :> Integer;
    attribute def Alarm :> Integer;
    attribute def Reset;

    state Monitor {
        attribute exits : Integer = 0;

        entry; then idle;
        state idle {
            exit action {
                in level : Integer = warn.w ?? alarm.a;
                assign exits := exits * 100 + level;
            }
        }
        state warned;
        state alarmed;

        transition warn first idle accept w : Warning then warned;
        transition alarm first idle accept a : Alarm then alarmed;
        transition first warned accept Reset then idle;
        transition first alarmed then done;
    }
}
