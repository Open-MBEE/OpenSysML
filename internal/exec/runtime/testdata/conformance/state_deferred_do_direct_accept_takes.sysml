// A state deferring Ping whose do behavior accepts Ping at its own level, as a
// branch beside the keeping loop of the standard deferred-signal encoding: the
// DeferredKeeper annotation names the keeping accept, so the state's other
// accept of Ping — written at the same level, with no nested action around it —
// takes a Ping arriving while both are parked, whichever the schedule would
// run first. A Ping arriving once `take` has passed is kept, and replayed when
// `busy` exits.
package Test {
    private import ScalarValues::Integer;
    item def Ping;
    item def Go;

    state DoDirectAcceptTakes {
        attribute heard : Integer = 0;
        entry; then start;
        state start;
        state busy {
            @MigrationMetadata::DeferredEvent { ref :>> signal : Ping; }
            item deferred : Ping[*] ordered;
            do action buffer {
                first start then split;
                fork split;
                then take;
                then receive;
                #MigrationMetadata::DeferredKeeper action receive accept kept : Ping;
                then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
                then receive;
                action take accept p : Ping;
                then action count { assign heard := heard + 1; }
            }
            exit action flush {
                for kept in deferred { send kept to self; }
                then action clear { assign deferred := (); }
            }
        }
        state ready;
        succession first start then busy;
        transition first busy accept Go then ready;
        transition first ready accept Ping then done;
    }
}
