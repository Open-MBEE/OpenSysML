// A guard on a segment out of a choice is read within the compound transition's
// performance: `raise.l` names the payload the accepting segment bound, as the
// segment's effect does, so the branch is chosen by what the occurrence carried.
package StateChoiceGuardReadsAcceptingSegment {
    private import ScalarValues::*;

    attribute def Level :> Integer;

    state Machine {
        attribute chosen : Integer = 0;

        entry; then idle;
        state idle;
        choice pick;
        state high;
        state low;

        transition raise first idle accept l : Level then pick;
        transition up first pick if raise.l > 5 do assign chosen := raise.l then high;
        transition down first pick if raise.l <= 5 do assign chosen := -raise.l then low;
        transition first high then done;
        transition first low then done;
    }
}
