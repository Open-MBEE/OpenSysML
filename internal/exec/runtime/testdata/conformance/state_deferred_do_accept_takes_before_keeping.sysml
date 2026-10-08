// A state deferring Ping whose own do behavior also accepts Ping: the standard
// deferred-signal encoding runs the keeping accept loop beside the state's own
// work as branches of one do action, and a Ping arriving while `work` is parked
// at its accept is consumed by `work`, not kept — an occurrence is deferred only
// when nothing of the state consumes it. A Ping arriving once `work` has passed
// its accept is kept, and replayed when `busy` exits.
package Test {
    private import ScalarValues::Integer;
    item def Ping;
    item def Go;

    state DoAcceptBeforeKeeping {
        attribute heard : Integer = 0;
        entry; then start;
        state start;
        state busy {
            @MigrationMetadata::DeferredEvent { ref :>> signal : Ping; }
            item deferred : Ping[*] ordered;
            do action buffer {
                first start then split;
                fork split;
                then receive;
                then work;
                #MigrationMetadata::DeferredKeeper action receive accept kept : Ping;
                then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
                then receive;
                action work {
                    action take accept p : Ping;
                    then action count { assign heard := heard + 1; }
                }
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
