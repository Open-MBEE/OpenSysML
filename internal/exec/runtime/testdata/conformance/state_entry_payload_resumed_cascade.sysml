// A composite that is not run-to-completion pauses its entry cascade at the
// boundary; the substate entered when the cascade resumes still reads the
// payload of the transition that entered the composite, `raise.l`, in its entry
// and its do behavior: both are performed within that transition's firing.
package StateEntryPayloadResumedCascade {
    private import ScalarValues::*;

    attribute def Level :> Integer;

    state Monitor {
        attribute entered : Integer = 0;
        attribute did : Integer = 0;

        entry; then idle;
        state idle;
        state work {
            attribute :>> isRunToCompletion = false;
            entry; then inner;
            state inner {
                entry action { in level : Integer = raise.l ?? 99; assign entered := level; }
                do { in level : Integer = raise.l ?? 99; assign did := level; }
            }
        }

        transition raise first idle accept l : Level then work;
    }
}
