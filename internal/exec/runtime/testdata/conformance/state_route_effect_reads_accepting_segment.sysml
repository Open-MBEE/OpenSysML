// A compound transition's later segments run within the performance that
// accepted the occurrence: the effects past a choice and a junction, like the exit
// before them and the entry they reach, read the payload off the accepting segment.
package StateRouteEffectReadsAcceptingSegment {
    private import ScalarValues::*;

    attribute def Level :> Integer;

    state Machine {
        attribute left : Integer = 0;
        attribute chosen : Integer = 0;
        attribute joined : Integer = 0;
        attribute entered : Integer = 0;

        entry; then idle;
        state idle {
            exit action {
                in level : Integer = raise.l;
                assign left := level;
            }
        }
        choice pick;
        junction meet;
        state high {
            entry action {
                in level : Integer = raise.l;
                assign entered := level;
            }
        }
        state low;

        transition raise first idle accept l : Level then pick;
        transition branch first pick if left > 5 do assign chosen := raise.l then meet;
        transition first pick then low;
        transition onward first meet do assign joined := raise.l then high;
        transition first high then done;
        transition first low then done;
    }
}
